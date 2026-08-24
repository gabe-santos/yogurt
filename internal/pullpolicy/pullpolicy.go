// Package pullpolicy computes when a Feed should next be checked. It is the
// project's second test seam: a pure function of the current time, the
// configured interval, the publisher's hints and the consecutive failure count,
// tested directly with table-driven cases rather than through HTTP, because
// proving time arithmetic over the API would need sleeps or a contrived clock
// dance for every case.
//
// It is empty until the polling ticket lands. Nothing here may touch the
// network, the database or the wall clock.
package pullpolicy
