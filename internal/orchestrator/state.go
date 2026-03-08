// Run state machine:
//
//	Created → Analyzing → Planning → Coding → Validating → Testing → Reviewing → Completed
package orchestrator

import (
	"fmt"

	"github.com/furkanbeydemir/orch/internal/models"
)

var validTransitions = map[models.RunStatus][]models.RunStatus{
	models.StatusCreated:    {models.StatusAnalyzing, models.StatusFailed},
	models.StatusAnalyzing:  {models.StatusPlanning, models.StatusFailed},
	models.StatusPlanning:   {models.StatusCoding, models.StatusFailed},
	models.StatusCoding:     {models.StatusValidating, models.StatusFailed},
	models.StatusValidating: {models.StatusTesting, models.StatusCoding, models.StatusFailed},
	models.StatusTesting:    {models.StatusReviewing, models.StatusCoding, models.StatusFailed},
	models.StatusReviewing:  {models.StatusCompleted, models.StatusCoding, models.StatusFailed},
	models.StatusCompleted:  {},
	models.StatusFailed:     {},
}

func Transition(state *models.RunState, target models.RunStatus) error {
	allowed, ok := validTransitions[state.Status]
	if !ok {
		return fmt.Errorf("bilinmeyen durum: %s", state.Status)
	}

	for _, valid := range allowed {
		if valid == target {
			state.Status = target
			return nil
		}
	}

	return fmt.Errorf("invalid state transition: %s → %s", state.Status, target)
}

func IsTerminal(status models.RunStatus) bool {
	return status == models.StatusCompleted || status == models.StatusFailed
}
