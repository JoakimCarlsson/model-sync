// Package elevenlabs parses ElevenLabs' models page and its API pricing page
// into the catalog model.
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
// The page opens with a card per flagship model, and that is the only place
// ElevenLabs states a display name or says what a model can do. Six of the
// eighteen models have one; the rest keep no name. A card's bullets are kept as
// written, under capabilities rather than features, because ElevenLabs
// enumerates nothing: it writes a sentence per capability with the size of the
// thing inside it, as "Speaker diarization, up to 32 speakers", and reducing
// that to a name would invent a vocabulary it never used.
package elevenlabs
