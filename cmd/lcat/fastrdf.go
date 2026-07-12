package main

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// fastNamespace is the FAST concept URI namespace vocab-subset detects to
// switch fetching to OCLC's linked-data shape.
const fastNamespace = "id.worldcat.org/fast/"

// isFASTURI reports whether a concept URI lives in the FAST namespace.
func isFASTURI(uri string) bool {
	return strings.Contains(uri, fastNamespace)
}

// fastRDFXMLToNT converts one FAST concept's RDF/XML (the fixed shape
// https://id.worldcat.org/fast/<id> serves under Accept:
// application/rdf+xml) into the SKOS N-Triples slice the subset pipeline
// reads: prefLabel, altLabel, broader (with the parent's label so hierarchy
// nodes stay labeled), and -- the crosswalk's first hop -- the concept's
// schema:sameAs link to its source LCSH heading emitted as skos:exactMatch.
// FAST is derived from LCSH, so its sameAs to the source heading is
// definitional identity, exactly what exactMatch asserts. This is a decoder
// for THAT endpoint's fixed generator output, not general RDF/XML.
func fastRDFXMLToNT(conceptURI string, body []byte) ([]byte, error) {
	dec := xml.NewDecoder(strings.NewReader(string(body)))
	conceptID := conceptURI[strings.LastIndex(conceptURI, "/")+1:]

	const (
		skosNS   = "http://www.w3.org/2004/02/skos/core#"
		schemaNS = "http://schema.org/"
		rdfNS    = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
		rdfsNS   = "http://www.w3.org/2000/01/rdf-schema#"
		lcshNS   = "id.loc.gov/authorities/subjects/"
	)

	var out strings.Builder
	triple := func(s, p, o string, literal bool) {
		if literal {
			fmt.Fprintf(&out, "<%s> <%s> %q@en .\n", s, p, o)
			return
		}
		fmt.Fprintf(&out, "<%s> <%s> <%s> .\n", s, p, o)
	}
	// resolve expands the endpoint's xml:base-relative rdf:about values
	// ("930306") to full FAST URIs, keeping absolute URIs as they are.
	resolve := func(about string) string {
		if strings.HasPrefix(about, "http://") || strings.HasPrefix(about, "https://") {
			return about
		}
		return "http://" + fastNamespace + about
	}

	// The walk tracks nesting: depth-1 Descriptions about the concept carry
	// its labels; a broader/sameAs element wraps a nested Description whose
	// rdf:about names the target.
	type frame struct {
		element string // local name of the open element ("prefLabel", "broader", ...)
		ns      string
	}
	var stack []frame
	inConcept := false
	var pending string // the relation ("broader"/"sameAs") awaiting its nested Description
	var pendingBroader string

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			var about string
			for _, a := range t.Attr {
				if a.Name.Space == rdfNS && a.Name.Local == "about" {
					about = a.Value
				}
			}
			if t.Name.Local == "Description" {
				switch {
				case inConcept && pending == "broader" && about != "":
					target := resolve(about)
					triple(conceptURI, skosNS+"broader", target, false)
					pendingBroader = target
				case inConcept && pending == "sameAs" && strings.Contains(about, lcshNS):
					triple(conceptURI, skosNS+"exactMatch", about, false)
				case len(stack) <= 1:
					// A top-level Description (only the rdf:RDF root open):
					// is it the concept we fetched?
					inConcept = about == conceptID || strings.HasSuffix(about, "/"+conceptID)
				}
			}
			stack = append(stack, frame{element: t.Name.Local, ns: t.Name.Space})
			if inConcept {
				switch {
				case t.Name.Space == skosNS && (t.Name.Local == "broader"):
					pending = "broader"
				case t.Name.Space == schemaNS && t.Name.Local == "sameAs":
					pending = "sameAs"
				}
			}
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			if t.Name.Local == "broader" || t.Name.Local == "sameAs" {
				pending = ""
				pendingBroader = ""
			}
		case xml.CharData:
			if !inConcept || len(stack) == 0 {
				continue
			}
			text := strings.TrimSpace(string(t))
			if text == "" {
				continue
			}
			top := stack[len(stack)-1]
			switch {
			case top.ns == skosNS && top.element == "prefLabel":
				triple(conceptURI, skosNS+"prefLabel", text, true)
			case top.ns == skosNS && top.element == "altLabel":
				triple(conceptURI, skosNS+"altLabel", text, true)
			case top.ns == rdfsNS && top.element == "label" && pendingBroader != "":
				// The broader parent's label rides along so the hierarchy
				// node is a heading, not a bare URI.
				triple(pendingBroader, skosNS+"prefLabel", text, true)
			}
		}
	}
	if out.Len() == 0 {
		return nil, fmt.Errorf("no SKOS statements decoded for %s -- not the FAST linked-data shape?", conceptURI)
	}
	return []byte(out.String()), nil
}
