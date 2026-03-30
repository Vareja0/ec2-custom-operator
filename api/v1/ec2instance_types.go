/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EC2instanceSpec defines the desired state of EC2instance
type EC2instanceSpec struct {
	// amiID: mydummyamiId
	// sshKey: my-keypair
	// type: t3-micro
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// foo is an example field of EC2instance. Edit ec2instance_types.go to remove/update
	// +optional
	AmiID             string            `json:"amiID"`
	SshKey            string            `json:"sshKey"`
	Type              string            `json:"type"`
	Storage           StorageConfig     `json:"storage"`
	Subnet            string            `json:"subnet"`
	Tags              map[string]string `json:"tags,omitempty"`
	InstanceName      string            `json:"instanceName"`
	SecurityGroups    []string          `json:"securityGroups,omitempty"`
	AssociatePublicIP bool              `json:"associatePublicIp,omitempty"`
	UserData          string            `json:"userData,omitempty"`
	AvaibilityZone    string            `json:"avaibilityZone,omitempty"`
	Region            string            `json:"region"`
}

type StorageConfig struct {
	RootVoume         VolumeConfig   `json:"rootVolume,omitempty"`
	AdditionalVolumes []VolumeConfig `json:"additionalVolumes,omitempty"`
}

type VolumeConfig struct {
	Size       int32  `json:"size"`
	Type       string `json:"type,omitempty"`
	DeviceName string `json:"deviceName,omitempty"`
	Encrypted  bool   `json:"encrypted,omitempty"`
}

// EC2instanceStatus defines the observed state of EC2instance.
type EC2instanceStatus struct {
	Status     string `json:"status,omitempty"`
	InstanceID string `json:"instanceID,omitempty"`
	PublicIP   string `json:"publicIP,omitempty"`
	PrivateIP  string `json:"privateIP,omitempty"`
	PublicDNS  string `json:"publicDNS,omitempty"`
	PrivateDNS string `json:"privateDNS,omitempty"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the EC2instance resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Instance ID",type="string",JSONPath=".status.instanceID"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.status"
// +kubebuilder:printcolumn:name="Public IP",type="string",JSONPath=".status.publicIP"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// EC2instance is the Schema for the ec2instances API
type EC2instance struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of EC2instance
	// +required
	Spec EC2instanceSpec `json:"spec"`

	// status defines the observed state of EC2instance
	// +optional
	Status EC2instanceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// EC2instanceList contains a list of EC2instance
type EC2instanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []EC2instance `json:"items"`
}

type CreatedInstanceInfo struct {
	InstanceID string `json:"instanceId"`
	PublicIP   string `json:"publicIP"`
	PrivateIP  string `json:"privateIP"`
	PublicDNS  string `json:"publicDNS"`
	PrivateDNS string `json:"privateDNS"`
	State      string `json:"state"`
}

type LiveInstanceSpec struct {
	State         string
	InstanceType  string
	AmiID         string
	SshKey        string
	AvailableZone string
	SubnetID      string
}

func init() {
	SchemeBuilder.Register(&EC2instance{}, &EC2instanceList{})
}
