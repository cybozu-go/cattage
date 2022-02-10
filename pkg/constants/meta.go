package constants

// MetaPrefix is the MetaPrefix for labels, annotations, and finalizers of Accurate.
const MetaPrefix = "cattage.cybozu.io/"

// Finalizer is the finalizer ID of Accurate.
const Finalizer = MetaPrefix + "finalizer"

const OwnerTenant = MetaPrefix + "tenant"

const OwnerAppNamespace = MetaPrefix + "owner-namespace"

const TenantFieldManager = MetaPrefix + "tenant-controller"
const ApplicationFieldManager = MetaPrefix + "application-controller"
const StatusFieldManager = ApplicationFieldManager + "/status"
const SpecFieldManager = ApplicationFieldManager + "/spec"
const ProjectFieldManager = ApplicationFieldManager + "/project"
