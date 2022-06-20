package model

import (
	"context"
	"encoding/json"
	"github.com/murphysecurity/murphysec/utils"
	"github.com/murphysecurity/murphysec/utils/must"
)

func GenerateIdeaErrorOutput(e error) string {
	iec := GetIdeaErrCode(e)
	return string(must.A(json.Marshal(struct {
		ErrCode IdeaErrCode `json:"err_code"`
		ErrMsg  string      `json:"err_msg"`
	}{ErrCode: iec, ErrMsg: e.Error()})))
}

type PluginOutput struct {
	ProjectName      string       `json:"project_name"`
	ErrCode          IdeaErrCode  `json:"err_code"`
	IssuesCount      int          `json:"issues_count,omitempty"`
	Comps            []PluginComp `json:"comps,omitempty"`
	IssuesLevelCount struct {
		Critical int `json:"critical,omitempty"`
		High     int `json:"high,omitempty"`
		Medium   int `json:"medium,omitempty"`
		Low      int `json:"low,omitempty"`
	} `json:"issues_level_count,omitempty"`
	TaskId            string         `json:"task_id,omitempty"`
	TotalContributors int            `json:"total_contributors"`
	ProjectId         string         `json:"project_id"`
	InspectErrors     []InspectError `json:"inspect_errors,omitempty"`
	DependenciesCount int            `json:"dependencies_count"`
	InspectReportUrl  string         `json:"inspect_report_url"`
}
type PluginComp struct {
	CompName           string               `json:"comp_name"`
	ShowLevel          int                  `json:"show_level"`
	MinFixedVersion    string               `json:"min_fixed_version"`
	DisposePlan        PluginCompFixList    `json:"dispose_plan"`
	Vulns              []VoVulnInfo         `json:"vulns"`
	Version            string               `json:"version"`
	License            *PluginCompLicense   `json:"license,omitempty"`
	Solutions          []PluginCompSolution `json:"solutions,omitempty"`
	IsDirectDependency bool                 `json:"is_direct_dependency"`
	Language           string               `json:"language"`
	FixType            string               `json:"fix_type"`
}

type PluginCompLicense struct {
	Level LicenseLevel `json:"level"`
	Spdx  string       `json:"spdx"`
}

type PluginCompFix struct {
	OldVersion      string `json:"old_version"`
	NewVersion      string `json:"new_version"`
	CompName        string `json:"comp_name"`
	UpdateSecScore  int    `json:"update_sec_score"`
	CompatibleScore int    `json:"compatible_score"`
}

type PluginCompFixList []PluginCompFix

func (this PluginCompFixList) MarshalJSON() ([]byte, error) {
	m := map[PluginCompFix]struct{}{}
	for _, it := range this {
		m[it] = struct{}{}
	}
	rs := make([]PluginCompFix, 0)
	for it := range m {
		rs = append(rs, it)
	}
	return must.A(json.Marshal(rs)), nil
}

type PluginCompSolution struct {
	Compatibility *int   `json:"compatibility,omitempty"`
	Description   string `json:"description"`
	Type          string `json:"type,omitempty"`
}

func GenerateIdeaOutput(c context.Context) string {
	ctx := UseScanTask(c)
	i := ctx.ScanResult
	type id struct {
		name    string
		version string
	}
	p := &PluginOutput{
		ProjectName: ctx.ProjectName,
		ErrCode:     0,
		IssuesCount: i.IssuesCompsCount,
		Comps:       []PluginComp{},
		TaskId:      i.TaskId,
		//InspectErrors:     ctx.InspectorError,
		TotalContributors: ctx.TotalContributors,
		ProjectId:         ctx.ProjectId,
		DependenciesCount: ctx.ScanResult.DependenciesCount,
		InspectReportUrl:  ctx.ScanResult.InspectReportUrl,
	}
	// merge module comps
	rs := map[id]PluginComp{}
	for _, mod := range i.Modules {
		for _, comp := range mod.Comps {
			cid := id{comp.CompName, comp.CompVersion}
			p := PluginComp{
				CompName:        comp.CompName,
				ShowLevel:       3,
				MinFixedVersion: comp.MinFixedVersion,
				DisposePlan:     PluginCompFixList{},
				Vulns:           comp.Vuls,
				Version:         comp.CompVersion,
				License:         nil,
				Solutions:       []PluginCompSolution{},
				Language:        mod.Language,
				FixType:         comp.FixType,
			}
			for _, it := range comp.MinFixedInfo {
				p.DisposePlan = append(p.DisposePlan, PluginCompFix{
					OldVersion:      it.OldVersion,
					NewVersion:      it.NewVersion,
					CompName:        it.Name,
					CompatibleScore: it.CompatibilityScore,
					UpdateSecScore:  it.SecurityScore,
				})
			}
			if comp.License != nil {
				p.License = &PluginCompLicense{
					Level: comp.License.Level,
					Spdx:  comp.License.Spdx,
				}
			}
			// Work-around to keep result consistency.
			if rs[cid].IsDirectDependency {
				p.IsDirectDependency = true
			} else {
				p.IsDirectDependency = comp.IsDirectDependency
			}
			for _, it := range comp.Solutions {
				p.Solutions = append(p.Solutions, PluginCompSolution{
					Compatibility: it.Compatibility,
					Description:   it.Description,
					Type:          it.Type,
				})
			}
			for _, it := range comp.Vuls {
				switch it.SuggestLevel {
				case SuggestLevelRecommend:
					p.ShowLevel = utils.MinInt(p.ShowLevel, 2)
				case SuggestLevelStrongRecommend:
					p.ShowLevel = utils.MinInt(p.ShowLevel, 1)
				}
			}
			rs[cid] = p
		}
	}
	for _, it := range rs {
		p.Comps = append(p.Comps, it)
	}
	// calc vulns
	{
		critical := map[string]struct{}{}
		high := map[string]struct{}{}
		medium := map[string]struct{}{}
		low := map[string]struct{}{}
		for _, it := range i.Modules {
			for _, comp := range it.Comps {
				for _, vul := range comp.Vuls {
					switch vul.Level {
					case VulnLevelCritical:
						critical[vul.VulnNo] = struct{}{}
					case VulnLevelHigh:
						high[vul.VulnNo] = struct{}{}
					case VulnLevelMedium:
						medium[vul.VulnNo] = struct{}{}
					case VulnLevelLow:
						low[vul.VulnNo] = struct{}{}
					}
				}
			}
		}
		p.IssuesLevelCount.Low = len(low)
		p.IssuesLevelCount.Medium = len(medium)
		p.IssuesLevelCount.High = len(high)
		p.IssuesLevelCount.Critical = len(critical)
	}
	return string(must.A(json.MarshalIndent(p, "", "  ")))
}