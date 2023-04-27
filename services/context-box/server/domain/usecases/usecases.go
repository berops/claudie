package usecases

import (
	"sync"

	"github.com/berops/claudie/services/context-box/server/domain/ports"
	"github.com/berops/claudie/services/context-box/server/utils"
)

type Usecases struct {
	MongoDB           ports.MongoDBPort
	configChangeMutex sync.Mutex

	// queue containing configs which needs to be processed by the builder microservice
	builderQueue utils.Queue
	// queue containing configs which needs to be processed by the scheduler microservice
	schedulerQueue utils.Queue
}
