package meta

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/m-lab/ndt-server/metrics"
	ndt5metrics "github.com/m-lab/ndt-server/ndt5/metrics"
	"github.com/m-lab/ndt-server/ndt5/protocol"
)

// maxClientMessages is the maximum allowed messages we will accept from a client.
var maxClientMessages = 20

// ArchivalData contains all meta data reported by the client.
type ArchivalData map[string]string

// ManageTest runs the meta tests. If the given ctx is canceled or the meta test
// takes longer than 15sec, then ManageTest will return after the next ReceiveMessage.
// The given protocolMessager should have its own connection timeout to prevent
// "slow drip" clients holding the connection open indefinitely.
func ManageTest(ctx context.Context, m protocol.Messager) (ArchivalData, error) {
	localCtx, localCancel := context.WithTimeout(ctx, 15*time.Second)
	defer localCancel()

	var err error
	var message []byte
	results := map[string]string{}

	err = m.SendMessage(protocol.TestPrepare, []byte{})
	if err != nil {
		log.Println("META TestPrepare:", err)
		metrics.ErrorCount.WithLabelValues("meta", "TestPrepare").Inc()
		return nil, err
	}
	err = m.SendMessage(protocol.TestStart, []byte{})
	if err != nil {
		log.Println("META TestStart:", err)
		metrics.ErrorCount.WithLabelValues("meta", "TestStart").Inc()
		return nil, err
	}
	count := 0
	for count < maxClientMessages && localCtx.Err() == nil {
		message, err = m.ReceiveMessage(protocol.TestMsg)
		if string(message) == "" || err != nil {
			break
		}
		count++

		s := strings.SplitN(string(message), ":", 2)
		if len(s) != 2 {
			continue
		}
		name := strings.TrimSpace(s[0])
		if len(name) > 63 {
			name = name[:63]
		}
		value := strings.TrimSpace(s[1])
		if len(value) > 255 {
			value = value[:255]
		}
		results[name] = value
	}
	if localCtx.Err() != nil {
		log.Println("META context error:", localCtx.Err())
		metrics.ErrorCount.WithLabelValues("meta", "context.Err").Inc()
		return nil, localCtx.Err()
	}
	if err != nil {
		log.Println("Error reading JSON message:", err)
		metrics.ErrorCount.WithLabelValues("meta", "ReceiveMessage").Inc()
		return nil, err
	}
	// Count the number meta values sent by the client (when there are no errors).
	ndt5metrics.SubmittedMetaValues.Observe(float64(count))
	err = m.SendMessage(protocol.TestFinalize, []byte{})
	if err != nil {
		log.Println("META TestFinalize:", err)
		metrics.ErrorCount.WithLabelValues("meta", "TestFinalize").Inc()
		return nil, err
	}
	return results, nil
}
