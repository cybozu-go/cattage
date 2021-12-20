package constants

// MetaPrefix is the MetaPrefix for labels, annotations, and finalizers of Accurate.
const MetaPrefix = "multi-tenancy.cybozu.com/"

// Finalizer is the finalizer ID of Accurate.
const Finalizer = MetaPrefix + "finalizer"

const OwnerTenant = MetaPrefix + "owner-tenant"

const OwnerApplication = MetaPrefix + "owner-application"
const OwnerAppNamespace = MetaPrefix + "owner-app-namespace"

const FieldManager = MetaPrefix + "neco-tenant-controller"
