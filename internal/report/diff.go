package report

import (
	"attestara/internal/adapters"
	"attestara/internal/contracts"
)

// Diff reports newly observed vulnerability findings and evidence references
// removed from a later passport. It contains claims only and does not validate
// referenced bytes, bind reports to passports, or establish trust.
type Diff struct {
	NewVulnerabilities []adapters.VulnerabilityFinding `json:"new_vulnerabilities"`
	RemovedEvidence    []contracts.EvidenceReference   `json:"removed_evidence"`
}

type findingIdentity struct {
	vulnerabilityID string
	artifactName    string
	artifactVersion string
}

type evidenceIdentity struct {
	path   string
	digest string
}

// Compare compares the typed claims from two release snapshots. Findings are
// matched by vulnerability ID, package name, and package version; removed
// evidence is matched by path and digest. Results retain the corresponding
// input order and duplicate identities are reported once.
func Compare(before Passport, beforeVulnerabilities adapters.VulnerabilityReport, after Passport, afterVulnerabilities adapters.VulnerabilityReport) Diff {
	diff := Diff{
		NewVulnerabilities: make([]adapters.VulnerabilityFinding, 0),
		RemovedEvidence:    make([]contracts.EvidenceReference, 0),
	}
	previousFindings := make(map[findingIdentity]struct{}, len(beforeVulnerabilities.Findings))
	for _, finding := range beforeVulnerabilities.Findings {
		previousFindings[findingIdentity{finding.VulnerabilityID, finding.ArtifactName, finding.ArtifactVersion}] = struct{}{}
	}
	reportedFindings := make(map[findingIdentity]struct{})
	for _, finding := range afterVulnerabilities.Findings {
		identity := findingIdentity{finding.VulnerabilityID, finding.ArtifactName, finding.ArtifactVersion}
		if _, existed := previousFindings[identity]; existed {
			continue
		}
		if _, reported := reportedFindings[identity]; reported {
			continue
		}
		diff.NewVulnerabilities = append(diff.NewVulnerabilities, finding)
		reportedFindings[identity] = struct{}{}
	}

	currentEvidence := make(map[evidenceIdentity]struct{}, len(after.EvidenceReferences))
	for _, reference := range after.EvidenceReferences {
		currentEvidence[evidenceIdentity{reference.Path, reference.Digest}] = struct{}{}
	}
	reportedEvidence := make(map[evidenceIdentity]struct{})
	for _, reference := range before.EvidenceReferences {
		identity := evidenceIdentity{reference.Path, reference.Digest}
		if _, retained := currentEvidence[identity]; retained {
			continue
		}
		if _, reported := reportedEvidence[identity]; reported {
			continue
		}
		diff.RemovedEvidence = append(diff.RemovedEvidence, reference)
		reportedEvidence[identity] = struct{}{}
	}
	return diff
}
