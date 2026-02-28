package worker

import "github.com/hibiken/asynq"

func Register(mux *asynq.ServeMux, p *Processor) {
	mux.HandleFunc(TaskSendNotification, p.HandleSendNotification)
}
