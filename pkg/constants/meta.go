package constants

// MetaPrefix is the MetaPrefix for labels, annotations, and finalizers of Cattage.
const MetaPrefix = "cattage.cybozu.io/"

// Finalizer is the finalizer ID of Cattage.
const Finalizer = MetaPrefix + "finalizer"

const OwnerTenant = MetaPrefix + "tenant"

const OwnerAppNamespace = MetaPrefix + "owner-namespace"

const TenantFieldManager = MetaPrefix + "tenant-controller"
