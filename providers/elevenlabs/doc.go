// Package elevenlabs parses ElevenLabs' models page, its API pricing page, the
// help center page listing model identifiers and three endpoint references into
// the catalog model.
//
// The documentation talks in credits, which are a plan allowance rather than a
// price, and never quotes a model in dollars. The pricing page is the one place
// that does: speech per thousand characters, transcription per hour, and voice
// changing, music and sound effects per minute. It quotes those rates per
// family — one card reads "Flash / Turbo" and another "Multilingual v2 / v3" —
// so a rate is matched to identifiers by the fragment a family's members share,
// most specific first, which is what keeps Scribe v2 Realtime out of the Scribe
// v2 card. The same rate applies on every plan, so a price carries no
// dimensions.
//
// Three of the cards price products rather than models: Speech Engine, Voice
// Isolator and both versions of Dubbing name nothing the models page lists, and
// are skipped.
//
// What ElevenLabs does not publish is a dollar rate for voice design, for which
// the help center says only that a generation is charged on the characters of
// the preview text without saying at which rate, nor one for the first
// generation models the character limit table still lists. Those models are
// left unpriced.
//
// What the models page does publish is a model's languages and, for speech
// synthesis, the longest text a single request may carry, which differs
// eightfold across the range. Its identifiers also encode what a model does — a
// name containing sts changes one voice into another, ttv designs a voice from
// a description, scribe transcribes — so the kind is read from the identifier,
// and with it the direction: every kind here is a pairing of text and audio,
// and which way round it runs is the whole of what the identifier says.
//
// A languages cell can name another model rather than a language, as "All
// eleven_multilingual_v2 languages plus: hu, no, vi". That is a reference, so
// the model it names supplies the rest of the list rather than being recorded
// as a language of its own.
//
// The page opens with a card per flagship model, and those cards are where
// ElevenLabs writes what a model can do. Six of the eighteen models have one.
// The names of the rest are on a help center page pairing a display name with
// an identifier, which the parser reads for nothing else: a row there naming an
// identifier the models page never listed is ignored rather than turning a name
// into a model. Where both name a model the card's wording wins, being the
// fuller of the two, so eleven_multilingual_v2 is "Eleven Multilingual v2"
// rather than "Multilingual v2". That page names five models the cards leave
// out, three of them still served. Seven are named nowhere at all: the two
// voice design models, the sound effects model, Music v1, Scribe v1 and the two
// first generation models the character limit table still lists.
//
// A card's bullets are sentences rather than capability names. ElevenLabs
// enumerates nothing: it writes a sentence per capability with the size of the
// thing inside it, as "Speaker diarization, up to 32 speakers". Each is split
// where that sentence divides, into the capability it names and the bound it
// states, so the capability list holds capabilities and the bound is a number
// a consumer can compare. "40,000 character limit" is a bound and no
// capability at all; "Accurate transcription in 90+ languages" is a count of
// languages, which is not the list of them the models table already carries;
// and the delay a card quotes is kept as written, footnote marker and all,
// because ElevenLabs states it as an approximation and rounding that away
// would claim a precision it did not give.
//
// A bullet that states no fact of any of those kinds is dropped. Several are
// marketing rather than specification, as "Most stable on long-form
// generations" is, and a capability list is the wrong place for a sentence
// nothing can be derived from.
//
// The endpoint references for voice design, sound effects and speech to text
// are read for the one thing they say that no other document does: which of an
// endpoint's models an ability belongs to. ElevenLabs writes that as a
// restriction on a request parameter, as "Only supported when using the
// eleven_ttv_v3 model", and only a parameter carrying such a restriction is
// read, because the restriction is what makes it a fact about a model rather
// than about the API. The parameter is recorded as the capability it names,
// never as its own name. The same pages state the longest text the voice design
// endpoint accepts, which belongs to both the models its model_id enumerates
// and is the same per-request character limit the models page states for speech
// synthesis.
//
// A context window in tokens is not something any of these models has. Every
// one of them is a pairing of text and audio with no token vocabulary of its
// own, and the bound ElevenLabs does publish, on the characters one request may
// carry, is recorded under its own key rather than dressed up as one.
package elevenlabs
