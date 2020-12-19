package main

import (
	"cont/api"
	"context"
)

func (s *server) Ps(ctx context.Context, empty *api.Empty) (*api.ActiveProcesses, error) {
	processes := s.listProcesses()
	result := &api.ActiveProcesses{Processes: processes}
	return result, nil
}

func (s *server) listProcesses() []*api.Process {
	processes := make([]*api.Process, 0, len(currentlyRunning))
	for id, c := range currentlyRunning {
		processes = append(processes, &api.Process{
			Id:   id.String(),
			Name: c.Name,
			Cmd:  c.Command,
			Pid:  int64(c.Cmd.Process.Pid),
		})
	}
	return processes
}
