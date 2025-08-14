package helpers

import (
	"log"
	"time"
)

func SimpleEventIO(event string, in, out any, startedAt time.Time) {
	log.Printf("[%s] %s: \n\tin:%+v \n\tout:%+v\n", event, time.Since(startedAt), in, out)
}
