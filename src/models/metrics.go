package models

import (
	"sync"
	"time"
)

// Metrics holds all bot metrics
type Metrics struct {
	mu                sync.RWMutex
	StartTime         time.Time
	MessagesProcessed int64
	CommandsProcessed int64
	ErrorsCount       int64
	LastMessageTime   time.Time
	ResponseTimes     []time.Duration
	MaxResponseTime   time.Duration
	MinResponseTime   time.Duration
	TotalResponseTime time.Duration
}

// NewMetrics creates new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		StartTime:       time.Now(),
		MinResponseTime: time.Duration(0), // Initialize with zero
	}
}

// RecordMessage records message processing
func (m *Metrics) RecordMessage() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.MessagesProcessed++
	m.LastMessageTime = time.Now()
}

// RecordCommand records command execution
func (m *Metrics) RecordCommand() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.CommandsProcessed++
}

// RecordError records error occurrence
func (m *Metrics) RecordError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.ErrorsCount++
}

// RecordResponseTime records response time
func (m *Metrics) RecordResponseTime(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.ResponseTimes = append(m.ResponseTimes, duration)
	m.TotalResponseTime += duration
	
	if duration > m.MaxResponseTime {
		m.MaxResponseTime = duration
	}
	
	// Set min response time only if it's the first record or smaller than current
	if m.MinResponseTime == 0 || duration < m.MinResponseTime {
		m.MinResponseTime = duration
	}
	
	// Limit records to save memory
	if len(m.ResponseTimes) > 1000 {
		m.ResponseTimes = m.ResponseTimes[1:]
	}
}

// GetStats returns current statistics
func (m *Metrics) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	uptime := time.Since(m.StartTime)
	avgResponseTime := time.Duration(0)
	
	if len(m.ResponseTimes) > 0 {
		avgResponseTime = m.TotalResponseTime / time.Duration(len(m.ResponseTimes))
	}
	
	// Format response times properly
	maxTime := "0s"
	minTime := "0s"
	if m.MaxResponseTime > 0 {
		maxTime = m.MaxResponseTime.String()
	}
	if m.MinResponseTime > 0 {
		minTime = m.MinResponseTime.String()
	}
	
	return map[string]interface{}{
		"uptime":            uptime.String(),
		"messages_processed": m.MessagesProcessed,
		"commands_processed": m.CommandsProcessed,
		"errors_count":       m.ErrorsCount,
		"last_message_time":  m.LastMessageTime.Format("2006-01-02 15:04:05"),
		"avg_response_time":  avgResponseTime.String(),
		"max_response_time":  maxTime,
		"min_response_time":  minTime,
		"response_samples":   len(m.ResponseTimes),
	}
}
