// RDA media and carrier types as the LOC value vocabularies the crosswalk
// emits (id.loc.gov/vocabulary/mediaTypes and /carriers): IRI, human label,
// and MARC 337/338 code. Small closed lists, so they ship as data -- the
// editor renders stored IRIs as labels and offers a picker instead of a raw
// URL box. Unknown IRIs (rdaregistry numerics, local terms) fall back to the
// generic IRI display.

export interface RdaTerm {
  iri: string;
  label: string;
  code: string;
  /** Carrier grouping (the media category), for optgroup rendering. */
  group?: string;
}

const MEDIA_NS = "http://id.loc.gov/vocabulary/mediaTypes/";
const CARRIER_NS = "http://id.loc.gov/vocabulary/carriers/";

function terms(ns: string, group: string | undefined, defs: [string, string][]): RdaTerm[] {
  return defs.map(([code, label]) => ({ iri: ns + code, label, code, ...(group ? { group } : {}) }));
}

/** RDA media types (MARC 337). */
export const MEDIA_TYPES: RdaTerm[] = terms(MEDIA_NS, undefined, [
  ["s", "audio"],
  ["c", "computer"],
  ["h", "microform"],
  ["p", "microscopic"],
  ["g", "projected"],
  ["e", "stereographic"],
  ["n", "unmediated"],
  ["v", "video"],
  ["x", "other"],
  ["z", "unspecified"],
]);

/** RDA carrier types (MARC 338), grouped by media category. */
export const CARRIER_TYPES: RdaTerm[] = [
  ...terms(CARRIER_NS, "audio", [
    ["sg", "audio cartridge"],
    ["se", "audio cylinder"],
    ["sd", "audio disc"],
    ["si", "sound-track reel"],
    ["sq", "audio roll"],
    ["ss", "audiocassette"],
    ["st", "audiotape reel"],
    ["sw", "audio wire reel"],
    ["sz", "other audio carrier"],
  ]),
  ...terms(CARRIER_NS, "computer", [
    ["ck", "computer card"],
    ["cb", "computer chip cartridge"],
    ["cd", "computer disc"],
    ["ce", "computer disc cartridge"],
    ["ca", "computer tape cartridge"],
    ["cf", "computer tape cassette"],
    ["ch", "computer tape reel"],
    ["cr", "online resource"],
    ["cz", "other computer carrier"],
  ]),
  ...terms(CARRIER_NS, "microform", [
    ["ha", "aperture card"],
    ["he", "microfiche"],
    ["hf", "microfiche cassette"],
    ["hb", "microfilm cartridge"],
    ["hc", "microfilm cassette"],
    ["hd", "microfilm reel"],
    ["hj", "microfilm roll"],
    ["hh", "microfilm slip"],
    ["hg", "microopaque"],
    ["hz", "other microform carrier"],
  ]),
  ...terms(CARRIER_NS, "microscopic", [
    ["pp", "microscope slide"],
    ["pz", "other microscopic carrier"],
  ]),
  ...terms(CARRIER_NS, "projected image", [
    ["mc", "film cartridge"],
    ["mf", "film cassette"],
    ["mr", "film reel"],
    ["mo", "film roll"],
    ["gd", "filmslip"],
    ["gf", "filmstrip"],
    ["gc", "filmstrip cartridge"],
    ["gt", "overhead transparency"],
    ["gs", "slide"],
    ["mz", "other projected-image carrier"],
  ]),
  ...terms(CARRIER_NS, "stereographic", [
    ["eh", "stereograph card"],
    ["es", "stereograph disc"],
    ["ez", "other stereographic carrier"],
  ]),
  ...terms(CARRIER_NS, "unmediated", [
    ["no", "card"],
    ["nn", "flipchart"],
    ["na", "roll"],
    ["nb", "sheet"],
    ["nc", "volume"],
    ["nr", "object"],
    ["nz", "other unmediated carrier"],
  ]),
  ...terms(CARRIER_NS, "video", [
    ["vc", "video cartridge"],
    ["vf", "videocassette"],
    ["vd", "videodisc"],
    ["vr", "videotape reel"],
    ["vz", "other video carrier"],
  ]),
  ...terms(CARRIER_NS, "unspecified", [["zu", "unspecified"]]),
];

const byIRI = new Map<string, RdaTerm>([...MEDIA_TYPES, ...CARRIER_TYPES].map((t) => [t.iri, t]));

/** The known media/carrier term for an IRI, or undefined (generic display). */
export function rdaTerm(iri: string): RdaTerm | undefined {
  return byIRI.get(iri);
}
