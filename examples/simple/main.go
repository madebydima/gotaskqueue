package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/madebydima/gotaskqueue"
)

type EmailData struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type ReportData struct {
	UserID    string    `json:"user_id"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

func main() {
	if err := os.MkdirAll("static/css", 0755); err != nil {
		log.Printf("Warning: could not create static/css directory: %v", err)
	}
	if err := os.MkdirAll("static/js", 0755); err != nil {
		log.Printf("Warning: could not create static/js directory: %v", err)
	}

	queue, err := gotaskqueue.New(
		gotaskqueue.WithRedisAddr("localhost:6379"),
		gotaskqueue.WithNamespace("myapp"),
		gotaskqueue.WithMaxRetries(3),
	)
	if err != nil {
		log.Fatal("Failed to create queue:", err)
	}
	defer queue.Close()

	queue.StartDashboard(":8080")
	log.Println("📊 Dashboard available at: http://localhost:8080")

	worker := queue.NewWorker(
		gotaskqueue.WithConcurrency(2),
		gotaskqueue.WithPollInterval(time.Second*2),
	)

	worker.Handle("send_email", sendEmailHandler)
	worker.Handle("generate_report", generateReportHandler)
	worker.Handle("cleanup", cleanupHandler)

	go worker.Start()

	for i := range 10 {
		emailData := EmailData{
			To:      fmt.Sprintf("user%d@example.com", i),
			Subject: fmt.Sprintf("Test Email %d", i),
			Body:    fmt.Sprintf("This is test email %d", i),
		}

		taskID, err := queue.Enqueue("send_email", emailData)
		if err != nil {
			log.Printf("Failed to enqueue email task: %v", err)
		} else {
			log.Printf("Enqueued email task: %s", taskID)
		}

		if i%3 == 0 {
			reportData := ReportData{
				UserID:    fmt.Sprintf("user%d", i),
				StartDate: time.Now().AddDate(0, 0, -7),
				EndDate:   time.Now(),
			}

			taskID, err := queue.EnqueueDelayed("generate_report", reportData, time.Second*10)
			if err != nil {
				log.Printf("Failed to enqueue report task: %v", err)
			} else {
				log.Printf("Enqueued delayed report task: %s", taskID)
			}
		}
	}

	cleanupData := map[string]any{"reason": "nightly_cleanup"}
	taskID, err := queue.EnqueueWithRetry("cleanup", cleanupData, 5)
	if err != nil {
		log.Printf("Failed to enqueue cleanup task: %v", err)
	} else {
		log.Printf("Enqueued cleanup task with custom retry: %s", taskID)
	}

	time.Sleep(time.Second * 30)

	worker.Stop()

	stats, err := queue.GetStats()
	if err != nil {
		log.Printf("Failed to get stats: %v", err)
	} else {
		log.Printf("Final stats - Pending: %d, Processing: %d, Completed: %d, Failed: %d, Total: %d",
			stats.Pending, stats.Processing, stats.Completed, stats.Failed, stats.Total)
	}
}

func sendEmailHandler(task *gotaskqueue.Task) error {
	var emailData EmailData
	if err := task.UnmarshalData(&emailData); err != nil {
		return err
	}

	log.Printf("Sending email to: %s, Subject: %s", emailData.To, emailData.Subject)

	time.Sleep(time.Second * 2)

	// if time.Now().Unix()%3 == 0 {
	//     return fmt.Errorf("random email sending error")
	// }

	log.Printf("Email sent successfully to: %s", emailData.To)
	return nil
}

func generateReportHandler(task *gotaskqueue.Task) error {
	var reportData ReportData
	if err := task.UnmarshalData(&reportData); err != nil {
		return err
	}

	log.Printf("Generating report for user: %s, Period: %s to %s",
		reportData.UserID, reportData.StartDate.Format("2006-01-02"), reportData.EndDate.Format("2006-01-02"))

	time.Sleep(time.Second * 3)

	log.Printf("Report generated for user: %s", reportData.UserID)
	return nil
}

func cleanupHandler(task *gotaskqueue.Task) error {
	var data map[string]any
	if err := task.UnmarshalData(&data); err != nil {
		return err
	}

	reason, _ := data["reason"].(string)
	log.Printf("Performing cleanup: %s", reason)

	time.Sleep(time.Second * 1)
	log.Printf("Cleanup completed: %s", reason)
	return nil
}
