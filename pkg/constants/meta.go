package constants

// MetaPrefix is the MetaPrefix for labels, annotations, and finalizers of Accurate.
const MetaPrefix = "multi-tenancy.cybozu.com/"

// Finalizer is the finalizer ID of Accurate.
const Finalizer = MetaPrefix + "finalizer"

const OwnerTenant = MetaPrefix + "tenant"

const OwnerAppNamespace = MetaPrefix + "owner-namespace"

const FieldManager = MetaPrefix + "neco-tenant-controller"
const StatusFieldManager = FieldManager + "/status"
const SpecFieldManager = FieldManager + "/spec"
const ProjectFieldManager = FieldManager + "/project"
