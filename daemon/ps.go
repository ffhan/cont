package daemon

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
	processes := make([]*api.Process, 0, len(s.currentlyRunning))
	for _, c := range s.getCurrentlyRunning() {
		processes = append(processes, &api.Process{
			Id:   c.Id.String(),
			Name: c.Name,
			Cmd:  c.Command,
			Pid:  int64(c.Cmd.Process.Pid),
		})
	}
	return processes
}
