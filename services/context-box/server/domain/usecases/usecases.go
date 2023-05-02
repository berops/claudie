package usecases

import (
	"sync"

	"github.com/berops/claudie/services/context-box/server/domain/ports"
	"github.com/berops/claudie/services/context-box/server/utils"
)

type Usecases struct {
	DB                ports.DBPort
	configChangeMutex sync.Mutex

	// queue containing configs which needs to be processed by the builder microservice
	builderQueue utils.Queue
	// queue containing configs which needs to be processed by the scheduler microservice
	schedulerQueue utils.Queue

	// Used for logging purposes
	// Logs are generated whenever elements are added/removed to/from the builder/scheduler queue
	builderLogQueue   []string
	schedulerLogQueue []string
}
