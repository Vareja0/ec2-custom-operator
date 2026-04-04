package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	ec2InstancesCreated = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ec2_instances_created_total",
		Help: "Total number of EC2 instances successfully created",
	})

	ec2InstancesDeleted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ec2_instances_deleted_total",
		Help: "Total number of EC2 instances successfully deleted",
	})

	ec2InstancesReplaced = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ec2_instances_replaced_total",
		Help: "Total number of EC2 instances replaced due to spec change",
	})

	ec2OperationErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ec2_operation_errors_total",
		Help: "Total number of errors per operation",
	}, []string{"operation"}) // operation: create, delete, describe

	ec2InstanceRunning = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ec2_instance_running",
		Help: "Whether the EC2 instance is in running state (1=running, 0=not running)",
	}, []string{"instance_id", "instance_name"})
)

func init() {
	metrics.Registry.MustRegister(
		ec2InstancesCreated,
		ec2InstancesDeleted,
		ec2InstancesReplaced,
		ec2OperationErrors,
		ec2InstanceRunning,
	)
}
