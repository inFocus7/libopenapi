// Copyright 2022 Dave Shanley / Quobix
// SPDX-License-Identifier: MIT

package index

import (
	"fmt"
	"github.com/pb33f/libopenapi/utils"
	"gopkg.in/yaml.v3"
	"net/url"
	"path/filepath"
	"strings"
)

// ResolvingError represents an issue the resolver had trying to stitch the tree together.
type ResolvingError struct {
	// ErrorRef is the error thrown by the resolver
	ErrorRef error

	// Node is the *yaml.Node reference that contains the resolving error
	Node *yaml.Node

	// Path is the shortened journey taken by the resolver
	Path string

	// CircularReference is set if the error is a reference to the circular reference.
	CircularReference *CircularReferenceResult
}

func (r *ResolvingError) Error() string {
	return fmt.Sprintf("%s: %s [%d:%d]", r.ErrorRef.Error(),
		r.Path, r.Node.Line, r.Node.Column)
}

// Resolver will use a *index.SpecIndex to stitch together a resolved root tree using all the discovered
// references in the doc.
type Resolver struct {
	specIndex              *SpecIndex
	resolvedRoot           *yaml.Node
	resolvingErrors        []*ResolvingError
	circularReferences     []*CircularReferenceResult
	ignoredPolyReferences  []*CircularReferenceResult
	ignoredArrayReferences []*CircularReferenceResult
	referencesVisited      int
	indexesVisited         int
	journeysTaken          int
	relativesSeen          int
	IgnorePoly             bool
	IgnoreArray            bool
}

// NewResolver will create a new resolver from a *index.SpecIndex
func NewResolver(index *SpecIndex) *Resolver {
	if index == nil {
		return nil
	}
	r := &Resolver{
		specIndex:    index,
		resolvedRoot: index.GetRootNode(),
	}
	index.resolver = r
	return r
}

// GetIgnoredCircularPolyReferences returns all ignored circular references that are polymorphic
func (resolver *Resolver) GetIgnoredCircularPolyReferences() []*CircularReferenceResult {
	return resolver.ignoredPolyReferences
}

// GetIgnoredCircularArrayReferences returns all ignored circular references that are arrays
func (resolver *Resolver) GetIgnoredCircularArrayReferences() []*CircularReferenceResult {
	return resolver.ignoredArrayReferences
}

// GetResolvingErrors returns all errors found during resolving
func (resolver *Resolver) GetResolvingErrors() []*ResolvingError {
	return resolver.resolvingErrors
}

// GetCircularErrors returns all circular reference errors found.
func (resolver *Resolver) GetCircularErrors() []*CircularReferenceResult {
	return resolver.circularReferences
}

// GetPolymorphicCircularErrors returns all circular errors that stem from polymorphism
func (resolver *Resolver) GetPolymorphicCircularErrors() []*CircularReferenceResult {
	var res []*CircularReferenceResult
	for i := range resolver.circularReferences {
		if !resolver.circularReferences[i].IsInfiniteLoop {
			continue
		}
		if !resolver.circularReferences[i].IsPolymorphicResult {
			continue
		}
		res = append(res, resolver.circularReferences[i])
	}
	return res
}

// GetNonPolymorphicCircularErrors returns all circular errors that DO NOT stem from polymorphism
func (resolver *Resolver) GetNonPolymorphicCircularErrors() []*CircularReferenceResult {
	var res []*CircularReferenceResult
	for i := range resolver.circularReferences {
		if !resolver.circularReferences[i].IsInfiniteLoop {
			continue
		}

		if !resolver.circularReferences[i].IsPolymorphicResult {
			res = append(res, resolver.circularReferences[i])
		}
	}
	return res
}

// IgnorePolymorphicCircularReferences will ignore any circular references that are polymorphic (oneOf, anyOf, allOf)
// This must be set before any resolving is done.
func (resolver *Resolver) IgnorePolymorphicCircularReferences() {
	resolver.IgnorePoly = true
}

// IgnoreArrayCircularReferences will ignore any circular references that stem from arrays. This must be set before
// any resolving is done.
func (resolver *Resolver) IgnoreArrayCircularReferences() {
	resolver.IgnoreArray = true
}

// GetJourneysTaken returns the number of journeys taken by the resolver
func (resolver *Resolver) GetJourneysTaken() int {
	return resolver.journeysTaken
}

// GetReferenceVisited returns the number of references visited by the resolver
func (resolver *Resolver) GetReferenceVisited() int {
	return resolver.referencesVisited
}

// GetIndexesVisited returns the number of indexes visited by the resolver
func (resolver *Resolver) GetIndexesVisited() int {
	return resolver.indexesVisited
}

// GetRelativesSeen returns the number of siblings (nodes at the same level) seen for each reference found.
func (resolver *Resolver) GetRelativesSeen() int {
	return resolver.relativesSeen
}

// Resolve will resolve the specification, everything that is not polymorphic and not circular, will be resolved.
// this data can get big, it results in a massive duplication of data. This is a destructive method and will permanently
// re-organize the node tree. Make sure you have copied your original tree before running this (if you want to preserve
// original data)
func (resolver *Resolver) Resolve() []*ResolvingError {

	visitIndex(resolver, resolver.specIndex)

	for _, circRef := range resolver.circularReferences {
		// If the circular reference is not required, we can ignore it, as it's a terminable loop rather than an infinite one
		if !circRef.IsInfiniteLoop {
			continue
		}

		resolver.resolvingErrors = append(resolver.resolvingErrors, &ResolvingError{
			ErrorRef: fmt.Errorf("infinite circular reference detected: %s", circRef.Start.Name),
			Node:     circRef.LoopPoint.Node,
			Path:     circRef.GenerateJourneyPath(),
		})
	}

	return resolver.resolvingErrors
}

// CheckForCircularReferences Check for circular references, without resolving, a non-destructive run.
func (resolver *Resolver) CheckForCircularReferences() []*ResolvingError {
	visitIndexWithoutDamagingIt(resolver, resolver.specIndex)
	for _, circRef := range resolver.circularReferences {
		// If the circular reference is not required, we can ignore it, as it's a terminable loop rather than an infinite one
		if !circRef.IsInfiniteLoop {
			continue
		}

		resolver.resolvingErrors = append(resolver.resolvingErrors, &ResolvingError{
			ErrorRef:          fmt.Errorf("infinite circular reference detected: %s", circRef.Start.Name),
			Node:              circRef.LoopPoint.Node,
			Path:              circRef.GenerateJourneyPath(),
			CircularReference: circRef,
		})
	}
	// update our index with any circular refs we found.
	resolver.specIndex.SetCircularReferences(resolver.circularReferences)
	return resolver.resolvingErrors
}

func visitIndexWithoutDamagingIt(res *Resolver, idx *SpecIndex) {
	mapped := idx.GetMappedReferencesSequenced()
	mappedIndex := idx.GetMappedReferences()
	res.indexesVisited++
	for _, ref := range mapped {
		seenReferences := make(map[string]bool)
		var journey []*Reference
		res.journeysTaken++
		res.VisitReference(ref.Reference, seenReferences, journey, false)
	}
	schemas := idx.GetAllComponentSchemas()
	for s, schemaRef := range schemas {
		if mappedIndex[s] == nil {
			seenReferences := make(map[string]bool)
			var journey []*Reference
			res.journeysTaken++
			res.VisitReference(schemaRef, seenReferences, journey, false)
		}
	}
	//for _, c := range idx.GetChildren() {
	//	visitIndexWithoutDamagingIt(res, c)
	//}
}

func visitIndex(res *Resolver, idx *SpecIndex) {
	mapped := idx.GetMappedReferencesSequenced()
	mappedIndex := idx.GetMappedReferences()
	res.indexesVisited++
	for _, ref := range mapped {
		seenReferences := make(map[string]bool)
		var journey []*Reference
		res.journeysTaken++
		if ref != nil && ref.Reference != nil {
			ref.Reference.Node.Content = res.VisitReference(ref.Reference, seenReferences, journey, true)
		}
	}

	schemas := idx.GetAllComponentSchemas()
	for s, schemaRef := range schemas {
		if mappedIndex[s] == nil {
			seenReferences := make(map[string]bool)
			var journey []*Reference
			res.journeysTaken++
			schemaRef.Node.Content = res.VisitReference(schemaRef, seenReferences, journey, true)
		}
	}

	// map everything
	for _, sequenced := range idx.GetAllSequencedReferences() {
		locatedDef := mappedIndex[sequenced.Definition]
		if locatedDef != nil {
			if !locatedDef.Circular && locatedDef.Seen {
				sequenced.Node.Content = locatedDef.Node.Content
			}
		}
	}
}

// VisitReference will visit a reference as part of a journey and will return resolved nodes.
func (resolver *Resolver) VisitReference(ref *Reference, seen map[string]bool, journey []*Reference, resolve bool) []*yaml.Node {
	resolver.referencesVisited++
	if ref.Resolved || ref.Seen {
		return ref.Node.Content
	}

	journey = append(journey, ref)
	relatives := resolver.extractRelatives(ref, ref.Node, nil, seen, journey, resolve)

	seen = make(map[string]bool)

	seen[ref.Definition] = true
	for _, r := range relatives {
		// check if we have seen this on the journey before, if so! it's circular
		skip := false
		for i, j := range journey {
			if j.FullDefinition == r.FullDefinition {

				var foundDup *Reference
				foundRef, _ := resolver.specIndex.SearchIndexForReferenceByReference(r)
				if foundRef != nil {
					foundDup = foundRef
				}

				var circRef *CircularReferenceResult
				if !foundDup.Circular {
					loop := append(journey, foundDup)

					visitedDefinitions := make(map[string]bool)
					isInfiniteLoop, _ := resolver.isInfiniteCircularDependency(foundDup, visitedDefinitions, nil)

					isArray := false
					if r.ParentNodeSchemaType == "array" {
						isArray = true
					}
					circRef = &CircularReferenceResult{
						Journey:        loop,
						Start:          foundDup,
						LoopIndex:      i,
						LoopPoint:      foundDup,
						IsArrayResult:  isArray,
						IsInfiniteLoop: isInfiniteLoop,
					}

					if resolver.IgnoreArray && isArray {
						resolver.ignoredArrayReferences = append(resolver.ignoredArrayReferences, circRef)
					} else {
						resolver.circularReferences = append(resolver.circularReferences, circRef)
					}

					foundDup.Seen = true
					foundDup.Circular = true
				}
				skip = true
			}
		}

		if !skip {
			var original *Reference
			foundRef, _ := resolver.specIndex.SearchIndexForReferenceByReference(r)
			if foundRef != nil {
				original = foundRef
			}
			resolved := resolver.VisitReference(original, seen, journey, resolve)
			if resolve && !original.Circular {
				r.Node.Content = resolved // this is where we perform the actual resolving.
			}
			r.Seen = true
			ref.Seen = true
		}
	}
	ref.Resolved = true
	ref.Seen = true

	return ref.Node.Content
}

func (resolver *Resolver) isInfiniteCircularDependency(ref *Reference, visitedDefinitions map[string]bool, initialRef *Reference) (bool, map[string]bool) {
	if ref == nil {
		return false, visitedDefinitions
	}

	for refDefinition := range ref.RequiredRefProperties {
		r, _ := resolver.specIndex.SearchIndexForReference(refDefinition)
		if initialRef != nil && initialRef.Definition == r.Definition {
			return true, visitedDefinitions
		}
		visitedDefinitions[r.Definition] = true

		ir := initialRef
		if ir == nil {
			ir = ref
		}

		var isChildICD bool
		isChildICD, visitedDefinitions = resolver.isInfiniteCircularDependency(r, visitedDefinitions, ir)
		if isChildICD {
			return true, visitedDefinitions
		}
	}

	return false, visitedDefinitions
}

func (resolver *Resolver) extractRelatives(ref *Reference, node, parent *yaml.Node,
	foundRelatives map[string]bool,
	journey []*Reference, resolve bool) []*Reference {

	if len(journey) > 100 {
		return nil
	}

	var found []*Reference

	if len(node.Content) > 0 {
		for i, n := range node.Content {
			if utils.IsNodeMap(n) || utils.IsNodeArray(n) {

				found = append(found, resolver.extractRelatives(ref, n, node, foundRelatives, journey, resolve)...)
			}

			if i%2 == 0 && n.Value == "$ref" {

				if !utils.IsNodeStringValue(node.Content[i+1]) {
					continue
				}

				value := node.Content[i+1].Value
				var locatedRef *Reference

				var fullDef string
				var definition string

				// explode value
				exp := strings.Split(value, "#/")
				if len(exp) == 2 {
					definition = fmt.Sprintf("#/%s", exp[1])
					if exp[0] != "" {

						if strings.HasPrefix(exp[0], "http") {
							fullDef = value
						} else {

							if filepath.IsAbs(exp[0]) {
								fullDef = value

							} else {

								if strings.HasPrefix(ref.FullDefinition, "http") {

									// split the http URI into parts
									httpExp := strings.Split(ref.FullDefinition, "#/")

									u, _ := url.Parse(httpExp[0])
									abs, _ := filepath.Abs(filepath.Join(filepath.Dir(u.Path), exp[0]))
									u.Path = abs
									u.Fragment = ""
									fullDef = fmt.Sprintf("%s#/%s", u.String(), exp[1])

								} else {

									// split the referring ref full def into parts
									fileDef := strings.Split(ref.FullDefinition, "#/")

									// extract the location of the ref and build a full def path.
									abs, _ := filepath.Abs(filepath.Join(filepath.Dir(fileDef[0]), exp[0]))
									fullDef = fmt.Sprintf("%s#/%s", abs, exp[1])

								}
							}
						}
					} else {

						// local component, full def is based on passed in ref
						if strings.HasPrefix(ref.FullDefinition, "http") {

							// split the http URI into parts
							httpExp := strings.Split(ref.FullDefinition, "#/")

							// parse a URL from the full def
							u, _ := url.Parse(httpExp[0])

							// extract the location of the ref and build a full def path.
							fullDef = fmt.Sprintf("%s#/%s", u.String(), exp[1])

						} else {

							// split the full def into parts
							fileDef := strings.Split(ref.FullDefinition, "#/")
							fullDef = fmt.Sprintf("%s#/%s", fileDef[0], exp[1])

						}

					}
				} else {

					definition = value

					// if the reference is a http link
					if strings.HasPrefix(value, "http") {
						fullDef = value
					} else {

						if filepath.IsAbs(value) {
							fullDef = value
						} else {

							// split the full def into parts
							fileDef := strings.Split(ref.FullDefinition, "#/")

							// is the file def a http link?
							if strings.HasPrefix(fileDef[0], "http") {

								u, _ := url.Parse(fileDef[0])
								path, _ := filepath.Abs(filepath.Join(filepath.Dir(u.Path), exp[0]))
								u.Path = path
								fullDef = u.String()

							} else {

								fullDef, _ = filepath.Abs(filepath.Join(filepath.Dir(fileDef[0]), exp[0]))
							}
						}
					}
				}

				searchRef := &Reference{
					Definition:     definition,
					FullDefinition: fullDef,
					RemoteLocation: ref.RemoteLocation,
					IsRemote:       true,
				}

				locatedRef, _ = resolver.specIndex.SearchIndexForReferenceByReference(searchRef)

				if locatedRef == nil {
					_, path := utils.ConvertComponentIdIntoFriendlyPathSearch(value)
					err := &ResolvingError{
						ErrorRef: fmt.Errorf("cannot resolve reference `%s`, it's missing", value),
						Node:     n,
						Path:     path,
					}
					resolver.resolvingErrors = append(resolver.resolvingErrors, err)
					continue
				}

				schemaType := ""
				if parent != nil {
					_, arrayTypevn := utils.FindKeyNodeTop("type", parent.Content)
					if arrayTypevn != nil {
						if arrayTypevn.Value == "array" {
							schemaType = "array"
						}
					}
				}

				locatedRef.ParentNodeSchemaType = schemaType
				found = append(found, locatedRef)
				foundRelatives[value] = true
			}

			if i%2 == 0 && n.Value != "$ref" && n.Value != "" {

				if n.Value == "allOf" ||
					n.Value == "oneOf" ||
					n.Value == "anyOf" {

					// if this is a polymorphic link, we want to follow it and see if it becomes circular
					if utils.IsNodeMap(node.Content[i+1]) { // check for nested items
						// check if items is present, to indicate an array
						if _, v := utils.FindKeyNodeTop("items", node.Content[i+1].Content); v != nil {
							if utils.IsNodeMap(v) {
								if d, _, l := utils.IsNodeRefValue(v); d {

									// create full definition lookup based on ref.
									def := l
									exp := strings.Split(l, "#/")
									if len(exp) == 2 {
										if exp[0] != "" {
											if !strings.HasPrefix(exp[0], "http") {
												if !filepath.IsAbs(exp[0]) {

													if strings.HasPrefix(ref.FullDefinition, "http") {

														u, _ := url.Parse(ref.FullDefinition)
														p, _ := filepath.Abs(filepath.Join(filepath.Dir(u.Path), exp[0]))
														u.Path = p
														u.Fragment = ""
														def = fmt.Sprintf("%s#/%s", u.String(), exp[1])

													} else {

														fd := strings.Split(ref.FullDefinition, "#/")
														abs, _ := filepath.Abs(filepath.Join(filepath.Dir(fd[0]), exp[0]))
														def = fmt.Sprintf("%s#/%s", abs, exp[1])
													}
												}
											} else {
												if len(exp[1]) > 0 {
													def = l
												} else {
													def = exp[0]
												}
											}
										} else {
											if strings.HasPrefix(ref.FullDefinition, "http") {
												u, _ := url.Parse(ref.FullDefinition)
												u.Fragment = ""
												def = fmt.Sprintf("%s#/%s", u.String(), exp[1])

											} else {
												if strings.HasPrefix(ref.FullDefinition, "#/") {
													def = fmt.Sprintf("#/%s", exp[1])
												} else {
													fdexp := strings.Split(ref.FullDefinition, "#/")
													def = fmt.Sprintf("%s#/%s", fdexp[0], exp[1])
												}
											}
										}
									} else {

										if strings.HasPrefix(l, "http") {
											def = l
										} else {
											if filepath.IsAbs(l) {
												def = l
											} else {

												// check if were dealing with a remote file
												if strings.HasPrefix(ref.FullDefinition, "http") {

													// split the url.
													u, _ := url.Parse(ref.FullDefinition)
													abs, _ := filepath.Abs(filepath.Join(filepath.Dir(u.Path), l))
													u.Path = abs
													u.Fragment = ""
													def = u.String()
												} else {
													lookupRef := strings.Split(ref.FullDefinition, "#/")
													abs, _ := filepath.Abs(filepath.Join(filepath.Dir(lookupRef[0]), l))
													def = abs
												}
											}
										}
									}

									mappedRefs, _ := resolver.specIndex.SearchIndexForReference(def)
									if mappedRefs != nil && !mappedRefs.Circular {
										circ := false
										for f := range journey {
											if journey[f].FullDefinition == mappedRefs.FullDefinition {
												circ = true
												break
											}
										}
										if !circ {
											resolver.VisitReference(mappedRefs, foundRelatives, journey, resolve)
										} else {
											loop := append(journey, mappedRefs)
											circRef := &CircularReferenceResult{
												Journey:             loop,
												Start:               mappedRefs,
												LoopIndex:           i,
												LoopPoint:           mappedRefs,
												PolymorphicType:     n.Value,
												IsPolymorphicResult: true,
											}

											mappedRefs.Seen = true
											mappedRefs.Circular = true
											if resolver.IgnorePoly {
												resolver.ignoredPolyReferences = append(resolver.ignoredPolyReferences, circRef)
											} else {
												resolver.circularReferences = append(resolver.circularReferences, circRef)
											}
										}
									}
								}
							}
						}
					}
					// for array based polymorphic items
					if utils.IsNodeArray(node.Content[i+1]) { // check for nested items
						// check if items is present, to indicate an array
						for q := range node.Content[i+1].Content {
							v := node.Content[i+1].Content[q]
							if utils.IsNodeMap(v) {
								if d, _, l := utils.IsNodeRefValue(v); d {

									// create full definition lookup based on ref.
									def := l
									exp := strings.Split(l, "#/")
									if len(exp) == 2 {
										if exp[0] != "" {
											if !strings.HasPrefix(exp[0], "http") {
												if !filepath.IsAbs(exp[0]) {

													if strings.HasPrefix(ref.FullDefinition, "http") {

														u, _ := url.Parse(ref.FullDefinition)
														p, _ := filepath.Abs(filepath.Join(filepath.Dir(u.Path), exp[0]))
														u.Path = p
														def = fmt.Sprintf("%s#/%s", u.String(), exp[1])

													} else {
														abs, _ := filepath.Abs(filepath.Join(filepath.Dir(ref.FullDefinition), exp[0]))
														def = fmt.Sprintf("%s#/%s", abs, exp[1])
													}
												}
											} else {
												if len(exp[1]) > 0 {
													def = l
												} else {
													def = exp[0]
												}
											}

										} else {
											if strings.HasPrefix(ref.FullDefinition, "http") {
												u, _ := url.Parse(ref.FullDefinition)
												u.Fragment = ""
												def = fmt.Sprintf("%s#/%s", u.String(), exp[1])

											} else {
												if strings.HasPrefix(ref.FullDefinition, "#/") {
													def = fmt.Sprintf("#/%s", exp[1])
												} else {
													def = fmt.Sprintf("%s#/%s", ref.FullDefinition, exp[1])
												}
											}
										}
									} else {

										if strings.HasPrefix(l, "http") {
											def = l
										} else {
											if filepath.IsAbs(l) {
												def = l
											} else {

												// check if were dealing with a remote file
												if strings.HasPrefix(ref.FullDefinition, "http") {

													// split the url.
													u, _ := url.Parse(ref.FullDefinition)
													abs, _ := filepath.Abs(filepath.Join(filepath.Dir(u.Path), l))
													u.Path = abs
													u.Fragment = ""
													def = u.String()
												} else {
													lookupRef := strings.Split(ref.FullDefinition, "#/")
													abs, _ := filepath.Abs(filepath.Join(filepath.Dir(lookupRef[0]), l))
													def = abs
												}
											}
										}
									}

									mappedRefs, _ := resolver.specIndex.SearchIndexForReference(def)
									if mappedRefs != nil && !mappedRefs.Circular {
										circ := false
										for f := range journey {
											if journey[f].FullDefinition == mappedRefs.FullDefinition {
												circ = true
												break
											}
										}
										if !circ {
											resolver.VisitReference(mappedRefs, foundRelatives, journey, resolve)
										} else {
											loop := append(journey, mappedRefs)

											circRef := &CircularReferenceResult{
												Journey:             loop,
												Start:               mappedRefs,
												LoopIndex:           i,
												LoopPoint:           mappedRefs,
												PolymorphicType:     n.Value,
												IsPolymorphicResult: true,
											}

											mappedRefs.Seen = true
											mappedRefs.Circular = true
											if resolver.IgnorePoly {
												resolver.ignoredPolyReferences = append(resolver.ignoredPolyReferences, circRef)
											} else {
												resolver.circularReferences = append(resolver.circularReferences, circRef)
											}
										}
									}
								}
							}
						}
					}
					break
				}
			}
		}
	}
	resolver.relativesSeen += len(found)
	return found
}
