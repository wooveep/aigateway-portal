package portal

import (
	"context"
	"testing"

	clientK8s "higress-portal-backend/internal/client/k8s"
)

func TestStartBillingUsageConsumerLoopDeduplicatesByRedisBinding(t *testing.T) {
	service := &Service{
		billingLoops: map[string]struct{}{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	binding := clientK8s.AIQuotaBinding{
		UsageEventStream: billingDefaultUsageStream,
		Redis: clientK8s.AIQuotaRedisConfig{
			ServiceName: "redis-server-master.aigateway-system.svc.cluster.local",
			ServicePort: 6379,
			Database:    0,
		},
	}

	service.startBillingUsageConsumerLoop(ctx, binding)
	service.startBillingUsageConsumerLoop(ctx, binding)

	if got := len(service.billingLoops); got != 1 {
		t.Fatalf("billing consumer loops = %d, want 1", got)
	}
}
