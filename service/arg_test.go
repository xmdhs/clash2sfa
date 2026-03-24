package service

import (
	"testing"
)

func TestFilterTags(t *testing.T) {
	tags := []string{"HK-01", "HK-02", "JP-01", "US-01", "SG-01"}

	cases := []struct {
		name    string
		include string
		exclude string
		want    []string
	}{
		{
			name: "no filter",
			want: tags,
		},
		{
			name:    "include HK",
			include: "HK",
			want:    []string{"HK-01", "HK-02"},
		},
		{
			name:    "exclude HK",
			exclude: "HK",
			want:    []string{"JP-01", "US-01", "SG-01"},
		},
		{
			name:    "include HK|JP",
			include: "HK|JP",
			want:    []string{"HK-01", "HK-02", "JP-01"},
		},
		{
			name:    "include HK then exclude 02",
			include: "HK",
			exclude: "02",
			want:    []string{"HK-01"},
		},
		{
			name:    "include matches nothing",
			include: "AU",
			want:    []string{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := filterTags(tags, c.include, c.exclude)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("filterTags() = %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("filterTags()[%d] = %v, want %v", i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestFilterTagsInvalidRegex(t *testing.T) {
	_, err := filterTags([]string{"a"}, "[", "")
	if err == nil {
		t.Fatal("expected error for invalid include regex")
	}
	_, err = filterTags([]string{"a"}, "", "[")
	if err == nil {
		t.Fatal("expected error for invalid exclude regex")
	}
}

func TestUrlTestParser(t *testing.T) {
	tags := []string{"HK-01", "HK-02", "JP-01", "US-01"}

	cases := []struct {
		name      string
		outbounds []string
		wantNil   bool
		want      []string
	}{
		{
			name:      "no include/exclude directives -> nil (no-op)",
			outbounds: []string{"direct", "block"},
			wantNil:   true,
		},
		{
			name:      "include directive only",
			outbounds: []string{"include: HK"},
			want:      []string{"HK-01", "HK-02"},
		},
		{
			name:      "exclude directive only",
			outbounds: []string{"exclude: HK"},
			want:      []string{"JP-01", "US-01"},
		},
		{
			name:      "include with extra static tags",
			outbounds: []string{"direct", "include: HK"},
			want:      []string{"direct", "HK-01", "HK-02"},
		},
		{
			name:      "include and exclude combined",
			outbounds: []string{"include: HK|JP", "exclude: 02"},
			want:      []string{"HK-01", "JP-01"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := urlTestParser(c.outbounds, tags)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != len(c.want) {
				t.Fatalf("urlTestParser() = %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("urlTestParser()[%d] = %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}
