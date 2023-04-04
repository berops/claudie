package envs

import "os"

var (
	ContextBoxUri= os.Getenv("CONTEXT_BOX_HOSTNAME") + ":" + os.Getenv("CONTEXT_BOX_PORT")
)