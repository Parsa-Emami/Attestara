package attestara.release

default allow := false
allow if { input.evidence_present == true }
