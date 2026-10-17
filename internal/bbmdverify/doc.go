// Package bbmdverify ports tools/bacnet-bbmd-verify.py: a read-only BACnet/IP
// probe (directed Who-Is, broadcast Who-Is, Read-BDT/Read-FDT) that tells
// whether a device's I-Am crosses the router, i.e. whether Niagara can
// re-bind it after an IP change. Output, colours (always on, as in Python),
// JSON and exit codes match the Python tool.
package bbmdverify
