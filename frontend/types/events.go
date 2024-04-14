package types

import "github.com/ubccr/grendel/bmc"

type EventStruct struct {
	Time        string
	User        string
	Severity    string
	Message     string
	JobMessages []bmc.JobMessage
}
