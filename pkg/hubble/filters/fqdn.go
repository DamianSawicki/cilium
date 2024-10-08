// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Hubble

package filters

import (
	"context"
	"fmt"
	"regexp"
	"slices"

	flowpb "github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
)

func sourceFQDN(ev *v1.Event) []string {
	return ev.GetFlow().GetSourceNames()
}

func destinationFQDN(ev *v1.Event) []string {
	return ev.GetFlow().GetDestinationNames()
}

func filterByFQDNs(fqdnPatterns []string, getFQDNs func(*v1.Event) []string) (FilterFunc, error) {
	fqdnRegexp, err := compileFQDNPattern(fqdnPatterns)
	if err != nil {
		return nil, err
	}

	return func(ev *v1.Event) bool {
		return slices.ContainsFunc(getFQDNs(ev), func(fqdn string) bool {
			return fqdnRegexp.MatchString(fqdn)
		})
	}, nil
}

// filterByDNSQueries returns a FilterFunc that filters a flow by L7.DNS.query field.
// The filter function returns true if and only if the DNS query field matches any of
// the regular expressions.
func filterByDNSQueries(queryPatterns []string) (FilterFunc, error) {
	queries := make([]*regexp.Regexp, 0, len(queryPatterns))
	for _, pattern := range queryPatterns {
		query, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile regexp: %w", err)
		}
		queries = append(queries, query)
	}
	return func(ev *v1.Event) bool {
		dns := ev.GetFlow().GetL7().GetDns()
		if dns == nil {
			return false
		}
		return slices.ContainsFunc(queries, func(query *regexp.Regexp) bool {
			return query.MatchString(dns.Query)
		})
	}, nil
}

// FQDNFilter implements filtering based on FQDN information
type FQDNFilter struct{}

// OnBuildFilter builds a FQDN filter
func (f *FQDNFilter) OnBuildFilter(ctx context.Context, ff *flowpb.FlowFilter) ([]FilterFunc, error) {
	var fs []FilterFunc

	if ff.GetSourceFqdn() != nil {
		ff, err := filterByFQDNs(ff.GetSourceFqdn(), sourceFQDN)
		if err != nil {
			return nil, err
		}
		fs = append(fs, ff)
	}

	if ff.GetDestinationFqdn() != nil {
		ff, err := filterByFQDNs(ff.GetDestinationFqdn(), destinationFQDN)
		if err != nil {
			return nil, err
		}
		fs = append(fs, ff)
	}

	if ff.GetDnsQuery() != nil {
		dnsFilters, err := filterByDNSQueries(ff.GetDnsQuery())
		if err != nil {
			return nil, fmt.Errorf("invalid DNS query filter: %w", err)
		}
		fs = append(fs, dnsFilters)
	}

	return fs, nil
}
