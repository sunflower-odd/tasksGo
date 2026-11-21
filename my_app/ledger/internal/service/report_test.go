package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"ledger/internal/domain"
)

type reportExpenses struct {
	domain.ExpenseRepository
	categories func(context.Context) ([]string, error)
	total      func(context.Context, string, string, string) (float64, error)
}

func (r reportExpenses) GetUniqueCategories(ctx context.Context) ([]string, error) {
	return r.categories(ctx)
}

func (r reportExpenses) TotalByCategory(ctx context.Context, category, from, to string) (float64, error) {
	return r.total(ctx, category, from, to)
}

func TestReportCalculatesCategoriesConcurrently(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	started := make(chan string, 2)
	release := make(chan struct{})
	go func() {
		<-started
		<-started
		close(release)
	}()
	repo := reportExpenses{
		categories: func(received context.Context) ([]string, error) {
			if received != ctx {
				t.Error("category query received a different context")
			}
			return []string{"food", "transport"}, nil
		},
		total: func(received context.Context, category, from, to string) (float64, error) {
			if received != ctx || from != "2026-10-01" || to != "2026-10-31" {
				t.Error("unexpected query arguments")
			}
			started <- category
			select {
			case <-release:
			case <-received.Done():
				return 0, received.Err()
			}
			if category == "food" {
				return 125.5, nil
			}
			return 40, nil
		},
	}
	report, err := New(nil, repo, nil).GetReportSummary(ctx, "2026-10-01", "2026-10-31")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Totals) != 2 || report.Totals["food"] != 125.5 || report.Totals["transport"] != 40 {
		t.Fatalf("unexpected totals: %v", report.Totals)
	}
}

func TestReportWaitsForCanceledWorkers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	started := make(chan struct{}, 2)
	finished := make(chan struct{}, 2)
	go func() {
		<-started
		<-started
		cancel()
	}()
	repo := reportExpenses{
		categories: func(context.Context) ([]string, error) {
			return []string{"food", "transport"}, nil
		},
		total: func(ctx context.Context, _, _, _ string) (float64, error) {
			started <- struct{}{}
			<-ctx.Done()
			finished <- struct{}{}
			return 0, ctx.Err()
		},
	}
	_, err := New(nil, repo, nil).GetReportSummary(ctx, "2026-10-01", "2026-10-31")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if len(finished) != 2 {
		t.Fatal("service returned before all workers finished")
	}
}

func TestReportReturnsWorkerError(t *testing.T) {
	want := errors.New("database unavailable")
	repo := reportExpenses{
		categories: func(context.Context) ([]string, error) {
			return []string{"food"}, nil
		},
		total: func(context.Context, string, string, string) (float64, error) {
			return 0, want
		},
	}
	_, err := New(nil, repo, nil).GetReportSummary(context.Background(), "2026-10-01", "2026-10-31")
	if !errors.Is(err, want) {
		t.Fatalf("expected worker error, got %v", err)
	}
}

func TestReportWithoutCategories(t *testing.T) {
	repo := reportExpenses{
		categories: func(context.Context) ([]string, error) { return nil, nil },
	}
	report, err := New(nil, repo, nil).GetReportSummary(context.Background(), "2026-10-01", "2026-10-31")
	if err != nil || report.Totals == nil || len(report.Totals) != 0 {
		t.Fatalf("expected empty totals, got %+v, %v", report, err)
	}
}

// Проверяем, что heartbeat пишет во время расчёта и прекращает писать после завершения или отмены.
func TestReportHeartbeatStops(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		name := "completed"
		if canceled {
			name = "canceled"
		}
		t.Run(name, func(t *testing.T) {
			messages := make(heartbeatLog, 32)
			previousOutput := log.Writer()
			log.SetOutput(messages)
			t.Cleanup(func() { log.SetOutput(previousOutput) })
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			release := make(chan struct{})
			repo := reportExpenses{
				categories: func(context.Context) ([]string, error) { return []string{"food"}, nil },
				total: func(ctx context.Context, _, _, _ string) (float64, error) {
					select {
					case <-release:
						return 10, nil
					case <-ctx.Done():
						return 0, ctx.Err()
					}
				},
			}
			finished := make(chan error, 1)
			go func() {
				_, err := New(nil, repo, nil).GetReportSummary(ctx, "2026-10-01", "2026-10-31")
				finished <- err
			}()
			select {
			case msg := <-messages:
				if !strings.Contains(msg, "report calculation in progress") {
					t.Fatalf("unexpected heartbeat: %s", msg)
				}
			case <-ctx.Done():
				t.Fatal("heartbeat did not log during calculation")
			}
			if canceled {
				cancel()
			} else {
				close(release)
			}
			select {
			case err := <-finished:
				if canceled && !errors.Is(err, context.Canceled) {
					t.Fatalf("expected cancellation, got %v", err)
				}
				if !canceled && err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("calculation did not finish")
			}
			for len(messages) > 0 {
				<-messages
			}
			select {
			case msg := <-messages:
				t.Fatalf("heartbeat continued after calculation: %s", msg)
			case <-time.After(600 * time.Millisecond):
			}
		})
	}
}

type heartbeatLog chan string

func (messages heartbeatLog) Write(p []byte) (int, error) {
	select {
	case messages <- string(p):
	default:
	}
	return len(p), nil
}
