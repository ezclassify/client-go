package ezclassify

// clientSpecVersion is the version this client reports as its engine spec
// version. Its major.minor is the rest-api line this client is intended to
// communicate with; a different major.minor from the engine version returned
// by [Client.Ping] indicates a version skew that may cause subtle behavioral
// differences.
//
// Between releases it carries the development version of the next release.
// Every release sets it to the released module version. Do not edit it
// by hand.
const clientSpecVersion = "1.0.0-dev.30"
