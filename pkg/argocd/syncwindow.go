package argocd

// SyncWindows is a collection of sync windows in this project
type SyncWindows []*SyncWindow

// SyncWindow contains the kind, time, duration and attributes that are used to assign the syncWindows to apps
type SyncWindow struct {
	// Kind defines if the window allows or blocks syncs
	Kind string `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`
	// Schedule is the time the window will begin, specified in cron format
	Schedule string `json:"schedule,omitempty" protobuf:"bytes,2,opt,name=schedule"`
	// Duration is the amount of time the sync window will be open
	Duration string `json:"duration,omitempty" protobuf:"bytes,3,opt,name=duration"`
	// Applications contains a list of applications that the window will apply to
	Applications []string `json:"applications,omitempty" protobuf:"bytes,4,opt,name=applications"`
	// Namespaces contains a list of namespaces that the window will apply to
	Namespaces []string `json:"namespaces,omitempty" protobuf:"bytes,5,opt,name=namespaces"`
	// Clusters contains a list of clusters that the window will apply to
	Clusters []string `json:"clusters,omitempty" protobuf:"bytes,6,opt,name=clusters"`
	// ManualSync enables manual syncs when they would otherwise be blocked
	ManualSync bool `json:"manualSync,omitempty" protobuf:"bytes,7,opt,name=manualSync"`
	// TimeZone of the sync that will be applied to the schedule
	TimeZone string `json:"timeZone,omitempty" protobuf:"bytes,8,opt,name=timeZone"`
	// UseAndOperator use AND operator for matching applications, namespaces and clusters instead of the default OR operator
	UseAndOperator bool `json:"andOperator,omitempty" protobuf:"bytes,9,opt,name=andOperator"`
	// Description of the sync that will be applied to the schedule, can be used to add any information such as a ticket number for example
	Description string `json:"description,omitempty" protobuf:"bytes,10,opt,name=description"`
}
