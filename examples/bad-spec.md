# Beacon Pairing Protocol

Version 0.3 — draft for review. This example seeds one instance of most
mustlint rules; `mustlint check examples/bad-spec.md` walks through them.

The key words "MUST", "MUST NOT", "SHOULD", and "MAY" in this document
are to be interpreted as described in RFC 2119.

## Discovery

REQ-1: A beacon MUST broadcast a pairing frame every 30 seconds.

REQ-2: The controller MUST not accept unsigned pairing frames.

REQ-2: A controller SHALL respond to pairing frames within a reasonable time.

REQ-5: Controllers MAY NOT pair with more than 16 beacons.

## Sessions

Beacons WILL rotate session keys after each pairing.

The controller should log rejected frames.
