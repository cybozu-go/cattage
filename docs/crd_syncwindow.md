
### Custom Resources

* [SyncWindow](#syncwindow)

### Sub Resources

* [SyncWindowList](#syncwindowlist)
* [SyncWindowSetting](#syncwindowsetting)
* [SyncWindowSpec](#syncwindowspec)
* [SyncWindowStatus](#syncwindowstatus)

#### SyncWindow

SyncWindow is the Schema for the syncwindows API

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | metav1.ObjectMeta | false |
| spec |  | [SyncWindowSpec](#syncwindowspec) | false |
| status |  | [SyncWindowStatus](#syncwindowstatus) | false |

[Back to Custom Resources](#custom-resources)

#### SyncWindowList

SyncWindowList contains a list of SyncWindow

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| metadata |  | metav1.ListMeta | false |
| items |  | [][SyncWindow](#syncwindow) | true |

[Back to Custom Resources](#custom-resources)

#### SyncWindowSetting

SyncWindowSetting contains the kind, time, duration and attributes that are used to assign the syncWindows to apps

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| kind | Kind defines if the window allows or blocks syncs | string | false |
| schedule | Schedule is the time the window will begin, specified in cron format | string | false |
| duration | Duration is the amount of time the sync window will be open | string | false |
| applications | Applications contains a list of applications that the window will apply to | []string | false |
| namespaces | Namespaces contains a list of namespaces that the window will apply to | []string | false |
| clusters | Clusters contains a list of clusters that the window will apply to | []string | false |
| manualSync | ManualSync enables manual syncs when they would otherwise be blocked | bool | false |
| timeZone | TimeZone of the sync that will be applied to the schedule | string | false |
| andOperator | UseAndOperator use AND operator for matching applications, namespaces and clusters instead of the default OR operator | bool | false |
| description | Description of the sync that will be applied to the schedule, can be used to add any information such as a ticket number for example | string | false |

[Back to Custom Resources](#custom-resources)

#### SyncWindowSpec

SyncWindowSpec defines the desired state of SyncWindow

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| syncWindows | SyncWindows is a list of sync windows | SyncWindows | true |

[Back to Custom Resources](#custom-resources)

#### SyncWindowStatus

SyncWindowStatus defines the observed state of SyncWindow

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| conditions | Conditions is an array of conditions. | []metav1.Condition | false |

[Back to Custom Resources](#custom-resources)
