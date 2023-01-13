package saucecloud

import (
	"context"
	"encoding/base64"
	"os"
	"os/signal"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/hostedexec"
)

type state uint8

const (
	running state = iota
	stopping
)

type HostedExecRunner struct {
	Project       hostedexec.Project
	RunnerService hostedexec.Service
	state         state
}

func (r *HostedExecRunner) Run() (int, error) {
	suite := r.Project.Suites[0]

	metadata := make(map[string]string)
	metadata["name"] = suite.Name

	files, err := mapFiles(suite.Files)
	if err != nil {
		log.Err(err).Str("suite", suite.Name).Msg("Unable to read source files")
		return 1, err
	}

	log.Info().Str("image", suite.Image).Str("suite", suite.Name).Msg("Starting suite.")

	runner, err := r.RunnerService.TriggerRun(context.Background(), hostedexec.RunnerSpec{
		Container: hostedexec.Container{
			Name: suite.Image,
			Auth: hostedexec.Auth{
				User:  suite.ImagePullAuth.User,
				Token: suite.ImagePullAuth.Token,
			},
		},
		EntryPoint: suite.EntryPoint,
		Env:        mapEnv(suite.Env),
		Files:      files,
		Metadata:   metadata,
	})
	if err != nil {
		return 1, err
	}

	sigChan := r.registerInterruptOnSignal(runner.ID, suite.Name)
	defer unregisterSignalCapture(sigChan)

	log.Info().Str("image", suite.Image).Str("suite", suite.Name).Msg("Started suite.")
	run, err := r.PollRun(context.Background(), runner.ID)
	if err != nil {
		return 1, err
	}
	// TODO: What are the actual statuses?
	if run.Status == "Completed" {
		return 0, nil
	}

	return 1, nil
}

func (r *HostedExecRunner) registerInterruptOnSignal(runID string, suiteName string) chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func(c <-chan os.Signal, runID string, hr *HostedExecRunner) {
		for {
			sig := <-c
			if sig == nil {
				return
			}
			switch hr.state {
			case running:
				log.Info().Str("suite", suiteName).Msg("Stopping suite")
				err := hr.RunnerService.StopRun(context.Background(), runID)
				if err != nil {
					log.Warn().Err(err).Str("suite", suiteName).Msg("Unable to stop suite.")
				}
				println("\nStopping run. Waiting for all tests in progress to be stopped... (press Ctrl-c again to exit without waiting)\n")
				hr.state = stopping
			case stopping:
				os.Exit(1)
			}
		}
	}(sigChan, runID, r)
	return sigChan
}

func (r *HostedExecRunner) PollRun(ctx context.Context, id string) (hostedexec.RunnerDetails, error) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	deathclock := time.NewTimer(24 * time.Hour)
	defer deathclock.Stop()

	for {
		select {
		case <-ticker.C:
			r, err := r.RunnerService.GetRun(ctx, id)
			if err != nil {
				return hostedexec.RunnerDetails{}, err
			}
			if r.Status == "Succeeded" {
				return r, nil
			}
			log.Info().Str("runID", r.ID).Str("status", r.Status).Msg("Waiting for run to complete.")
		case <-deathclock.C:
			r, err := r.RunnerService.GetRun(ctx, id)
			if err != nil {
				return hostedexec.RunnerDetails{}, err
			}
			return r, nil
		}
	}
}

func mapEnv(env map[string]string) []hostedexec.EnvItem {
	var items []hostedexec.EnvItem
	for key, val := range env {
		items = append(items, hostedexec.EnvItem{
			Name:  key,
			Value: val,
		})
	}
	return items
}

func mapFiles(files []hostedexec.File) ([]hostedexec.FileData, error) {
	var items []hostedexec.FileData
	for _, f := range files {
		data, err := readFile(f.Src)
		if err != nil {
			return items, err
		}
		items = append(items, hostedexec.FileData{
			Path: f.Dst,
			Data: data,
		})
	}
	return items, nil
}

func readFile(path string) (string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.Strict().EncodeToString(bytes), nil
}
