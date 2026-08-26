// Package worker contains the background-process lifecycle. Actual queue
// consumption is introduced with SQS in milestone 3; for now the process stays
// alive and responds correctly to container shutdown signals.
package worker

import "context"

func Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}
