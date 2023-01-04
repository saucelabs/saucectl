package saucecloud

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/saucelabs/saucectl/internal/htexec"
)

type HostedRunner struct {
	Project       htexec.Project
	RunnerService htexec.Service
}

func (r *HostedRunner) Run() (int, error) {
	suite := r.Project.Suites[0]

	metadata := make(map[string]string)
	metadata["name"] = suite.Name

	runner, err := r.RunnerService.TriggerRun(context.Background(), htexec.RunnerSpec{
		Image:      suite.Image,
		EntryPoint: suite.EntryPoint,
		Env:        mapEnv(suite.Env),
		Files:      mapFiles(suite.Files),
		Artifacts:  suite.Artifacts,
		Metadata:   metadata,
	})
	if err != nil {
	}

	sigChan := r.registerInterruptOnSignal(runner.ID)
	defer unregisterSignalCapture(sigChan)

	fmt.Println("Started: ", runner.ID)
	run, err := r.PollRun(context.Background(), "et")
	if err != nil {
	}
	if run.Status == "Completed" {
		return 0, nil
	}

	return 0, nil
}

func (r *HostedRunner) registerInterruptOnSignal(runID string) chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func(c <-chan os.Signal, runID string) {
		sig := <-c
		if sig == nil {
			return
		}
		r.RunnerService.StopRun(context.Background(), runID)
	}(sigChan, runID)
	return sigChan
}

func (r *HostedRunner) PollRun(ctx context.Context, id string) (htexec.RunnerDetails, error) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	deathclock := time.NewTimer(24 * time.Hour)
	defer deathclock.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Println("Polling: ", id)
			r, err := r.RunnerService.GetRun(ctx, id)
			if err != nil {
				return htexec.RunnerDetails{}, err
			}
			fmt.Println("Run status: ", r.Status)
			if r.Status == "Completed" {
				return r, nil
			}
		case <-deathclock.C:
			r, err := r.RunnerService.GetRun(ctx, id)
			if err != nil {
				return htexec.RunnerDetails{}, err
			}
			return r, nil
		}
	}
}

func mapEnv(env map[string]string) []htexec.EnvItem {
	var items []htexec.EnvItem
	for key, val := range env {
		items = append(items, htexec.EnvItem{
			Name:  key,
			Value: val,
		})
	}
	return items
}

func mapFiles(files []htexec.File) []htexec.FileData {
	var items []htexec.FileData
	for _, f := range files {
		items = append(items, htexec.FileData{
			Path: f.Dst,
			Data: readFile(f.Src),
		})
	}
	return items
}

func readFile(path string) string {
	bytes, err := os.ReadFile(path)
	if err != nil {
		// TODO: Warn and ignore
	}
	return base64.StdEncoding.Strict().EncodeToString(bytes)
}
