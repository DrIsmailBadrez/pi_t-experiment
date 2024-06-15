package usecases

import (
	"time"

	"github.com/HannahMarsh/pi_t-experiment/internal/domain/interfaces"
	"github.com/HannahMarsh/pi_t-experiment/internal/domain/models"
)

type NodeService struct {
	repo     interfaces.NodeRepository
	interval time.Duration
}

func (s *NodeService) RegisterNode(node *models.Node) error {
	return s.repo.RegisterNode(node)
}

func (s *NodeService) Heartbeat(nodeID string) error {
	return s.repo.Heartbeat(nodeID)
}

func (s *NodeService) StartActions() {
	ticker := time.NewTicker(s.interval)
	for {
		select {
		case <-ticker.C:
			// Logic to start node actions for each run
		}
	}
}
