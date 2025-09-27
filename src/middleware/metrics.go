package middleware

import (
	"time"

	"gopkg.in/telebot.v3"
	"gobrev/src/models"
)

// MetricsMiddleware creates metrics collection middleware
func MetricsMiddleware(metrics *models.Metrics) telebot.MiddlewareFunc {
	return func(next telebot.HandlerFunc) telebot.HandlerFunc {
		return func(c telebot.Context) error {
			start := time.Now()
			
			// Record message processing
			metrics.RecordMessage()
			
			// Execute next handler
			err := next(c)
			
			// Record response time
			duration := time.Since(start)
			metrics.RecordResponseTime(duration)
			
			// Record error if occurred
			if err != nil {
				metrics.RecordError()
			}
			
			return err
		}
	}
}
