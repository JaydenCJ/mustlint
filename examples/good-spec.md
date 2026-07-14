# Beacon Pairing Protocol

Version 0.4 — the cleaned-up counterpart of `bad-spec.md`. It lints clean,
including under `--require-ids`.

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT",
"SHOULD", "SHOULD NOT", "RECOMMENDED", "NOT RECOMMENDED", "MAY", and
"OPTIONAL" in this document are to be interpreted as described in BCP 14
[RFC2119] [RFC8174] when, and only when, they appear in all capitals, as
shown here.

## Discovery

REQ-1: A beacon MUST broadcast a pairing frame every 30 seconds.

REQ-2: The controller MUST NOT accept unsigned pairing frames.

REQ-3: A controller MUST respond to pairing frames within 250 ms.

REQ-4: Controllers MUST NOT pair with more than 16 beacons; further
pairing frames MUST be answered with code `LIMIT`.

## Sessions

REQ-5: Beacons MUST rotate session keys after each pairing.

REQ-6: The controller SHOULD log rejected frames, unless the operator
disabled the audit log.
