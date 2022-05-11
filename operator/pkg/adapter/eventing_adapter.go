package adapter

type Eventing func(eventtype, reason, message string)
