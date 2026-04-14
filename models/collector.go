package models

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"sync"
	"time"
)

type DataCollector struct {
	config      *Config
	buffer      *CircularBuffer
	subscribers map[net.Conn]chan bool
	subsMu      sync.RWMutex
	stopChan    chan bool
	rtcDev      string
}

func NewDataCollector(cfg *Config) *DataCollector {
	return &DataCollector{
		config:      cfg,
		buffer:      NewCircularBuffer(cfg.BufferSize),
		subscribers: make(map[net.Conn]chan bool),
		stopChan:    make(chan bool),
		rtcDev:      "/dev/rtc0",
	}
}

func (dc *DataCollector) readMetric(path string) float64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return math.NaN()
	}

	var value float64
	fmt.Sscanf(string(data), "%f", &value)
	return value
}

func (dc *DataCollector) getCurrentTime() int64 {
	return time.Now().UnixMilli()
}

func (dc *DataCollector) collectData() DataPoint {
	timestamp := dc.getCurrentTime()
	values := make(map[string]float64)

	for name, path := range dc.config.DataSources {
		values[name] = dc.readMetric(path)
	}

	return DataPoint{
		Timestamp: timestamp,
		Values:    values,
	}
}

func (dc *DataCollector) poll() {
	ticker := time.NewTicker(time.Duration(dc.config.PollIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			point := dc.collectData()
			dc.buffer.Push(point)
			dc.notifySubscribers(point)
		case <-dc.stopChan:
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
	// Удаляем существующий сокет
	os.Remove(dc.config.UDSSocketPath)

	listener, err := net.Listen("unix", dc.config.UDSSocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on UDS: %v", err)
	}

	// Запускаем сбор данных
	go dc.poll()

	// Принимаем соединения
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				continue
			}
			go dc.handleConnection(conn)
		}
	}()

	return nil
}
