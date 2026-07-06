package models

import (
	"collector/utils"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type DataCollector struct {
	log         *utils.Logger
	config      *Config
	buffer      *CircularBuffer
	subscribers map[net.Conn]chan bool
	subsMu      sync.RWMutex
	stopChan    chan bool
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	listener    net.Listener
	rtcDev      string
}

func NewDataCollector(cfg *Config) *DataCollector {
	ctx, cancel := context.WithCancel(context.Background())
	return &DataCollector{
		log:         utils.GlobalLogger(),
		config:      cfg,
		buffer:      NewCircularBuffer(cfg.BufferSize),
		subscribers: make(map[net.Conn]chan bool),
		stopChan:    make(chan bool),
		ctx: ctx,
		cancel: cancel,
		rtcDev:      "/dev/rtc0",
	}
}

func (dc *DataCollector) readNumberMetric(path string) float64 {
	data, err := os.ReadFile(path)
	if err != nil {
        return math.NaN()
    }

	str := strings.TrimSpace(string(data))
	var value float64
	if _, err := fmt.Sscanf(str, "%f", &value); err != nil {
        return math.NaN()
    }
	return value
}

func (dc *DataCollector) readStringMetric(path string) string {
    data, err := os.ReadFile(path)
    if err != nil {
        return "NaN"
    }
    
    str := strings.TrimSpace(string(data))
    
    return str
}

func (dc *DataCollector) readMetric(name string, config SourceConfig) MetricValue {

	switch config.Type {
	case TypeNumber:
		return MetricValue{
			Type: TypeNumber,
			Number: dc.readNumberMetric(config.Path),
		}
	case TypeString:
		return MetricValue {
			Type: TypeString,
			String: dc.readStringMetric(config.Path),
		}
	default:
		return MetricValue{
			Type: TypeString,
			String: "NotSupportedType",
		}
	}
}

func (dc *DataCollector) getCurrentTime() int64 {
	return time.Now().UnixMilli()
}

func (dc *DataCollector) collectData() DataPoint {
	timestamp := dc.getCurrentTime()
	values := make(map[string]interface{})

	for name, sourceConfig := range dc.config.DataSources {
		values[name] = dc.readMetric(name, sourceConfig).ToInterface()
	}

	return DataPoint{
		Timestamp: timestamp,
		Values:    values,
	}
}

func (dc *DataCollector) poll() {
	defer dc.wg.Done()
	ticker := time.NewTicker(time.Duration(dc.config.PollIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			point := dc.collectData()
			dc.buffer.Push(point)
			dc.notifySubscribers(point)
		case <-dc.ctx.Done():
			return
		}
	}
}

func (dc *DataCollector) notifySubscribers(point DataPoint) {
	dc.subsMu.RLock()
	defer dc.subsMu.RUnlock()

	data, _ := json.Marshal(point)
	data = append(data, '\n')

	for conn, _ := range dc.subscribers {
		_, err := conn.Write(data)
		if err != nil {
			// Ошибка отправки, подписчик будет удален позже
		}
	}
}

func (dc *DataCollector) handleConnection(conn net.Conn) {
	defer dc.wg.Done()
	defer conn.Close()

	for {
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			dc.removeSubscriber(conn)
			return
		}

		command := string(buf[:n])
		switch command {
		case "GET\n", "GET\r\n":
			// Запрос всех буферизированных данных
			allData := dc.buffer.GetAll()
			response, _ := json.Marshal(allData)
			conn.Write(response)

		case "SUBSCRIBE\n", "SUBSCRIBE\r\n":
			dc.log.Info("Get subscriber!!!")
			// Подписка на поток данных
			dc.addSubscriber(conn)
			// Ожидаем отписки
			<-dc.waitForUnsubscribe(conn)
			dc.removeSubscriber(conn)
			return

		default:
			conn.Write([]byte("Unknown command\n"))
		}
	}
}

func (dc *DataCollector) addSubscriber(conn net.Conn) {
	dc.subsMu.Lock()
	defer dc.subsMu.Unlock()
	dc.subscribers[conn] = make(chan bool)
}

func (dc *DataCollector) removeSubscriber(conn net.Conn) {
	dc.subsMu.Lock()
	defer dc.subsMu.Unlock()
	delete(dc.subscribers, conn)
}

func (dc *DataCollector) waitForUnsubscribe(conn net.Conn) chan bool {
	ch := make(chan bool)
	go func() {
		buf := make([]byte, 1)
		conn.Read(buf) // Блокируемся до закрытия соединения
		close(ch)
	}()
	return ch
}

func (dc *DataCollector) Start() error {
	// Удаляем существующий сокет если существует
	_, err := os.Stat(dc.config.UDSSocketPath)
	if err == nil {
		err = os.Remove(dc.config.UDSSocketPath)
		if err != nil {
			return fmt.Errorf("can't delete old socket %s (%v)", dc.config.UDSSocketPath, err)
		}
	}
	
	dirpath := filepath.Dir(dc.config.UDSSocketPath)
	_, err = os.Stat(dirpath)
	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(dirpath, 0755)
		if err != nil {
			return fmt.Errorf("can't create dirpath %s (%v)", dirpath, err)
		}
	} 

	listener, err := net.Listen("unix", dc.config.UDSSocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on UDS: %v", err)
	}

	dc.listener = listener

	dc.wg.Add(1)

	// Запускаем сбор данных
	go dc.poll()

	// Принимаем соединения
	dc.wg.Add(1)
	go func() {
		defer dc.wg.Done()
		for {
			select {
			case <-dc.ctx.Done():
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					select {
					case <-dc.ctx.Done():
						return
					default:
						continue
					}
				}
				dc.wg.Add(1)
				go dc.handleConnection(conn)
		    }
		}
	}()

	return nil
}

func (dc *DataCollector) Stop() error {
    // Отменяем контекст
    dc.cancel()
    
    // Закрываем listener
    if dc.listener != nil {
        dc.listener.Close()
    }
    
    // Ждем завершения всех горутин с таймаутом
    done := make(chan struct{})
    go func() {
        dc.wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        return nil
    case <-time.After(30 * time.Second):
        return fmt.Errorf("timeout waiting for goroutines to stop")
    }
}
