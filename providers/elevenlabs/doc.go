// Package elevenlabs parses ElevenLabs' models page, its API pricing page, the
// help center page listing model identifiers, six endpoint references and four
// capability guides into the catalog model.
//
// The one document that would answer everything at once is the models endpoint.
// GET /v1/models returns a name, a description, a language list, a per-request
// character bound and a rate multiplier for every model ElevenLabs serves, and
// it is documented publicly, but a request carrying no key is answered with
// workspace_not_found rather than with the list. Everything here is therefore
// read from the documentation, which states the same facts across several pages
// and states some of them only for the models it markets.
//
// The documentation talks in credits, which are a plan allowance rather than a
// price, and never quotes a model in dollars. The pricing page is the one place
// that does: speech per thousand characters, transcription per hour, and voice
// changing, music and sound effects per minute. It quotes those rates per
// family — one card reads "Flash / Turbo" and another "Multilingual v2 / v3" —
// so a rate is matched to identifiers by the fragment a family's members share,
// most specific first, which is what keeps Scribe v2 Realtime out of the Scribe
// v2 card. The same rate applies on every plan, so a rate for the audio itself
// carries no dimensions.
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
// A cell is also written as a link where the list is long. ElevenLabs writes
// "70+ languages" against Eleven v3 over a link to the section further down the
// same page that names all seventy-four, and "90+ languages" against each of
// the three transcription models over a link to the section of the speech to
// text guide that names ninety-nine. A link is a reference of the same kind as
// a model name, so every section of every fetched document is indexed by the
// anchor a link reaches it by, and a cell holding a link takes its list from
// the section it points at. Where one anchor names two sections, as it does for
// Eleven v3 and Eleven v3 Conversational, the first wins, which is how a
// browser resolves one.
//
// The two forms of list are not written alike and are not made alike here. A
// cell naming its languages writes them as two letter codes and a section
// naming them writes them as three, so a model whose list came from a section
// carries three letter codes. Rewriting one into the other would be this
// package deciding what ElevenLabs meant.
//
// The deprecated table carries a fourth column the current one does not, naming
// the model to move to. It is the only forward pointer ElevenLabs publishes for
// something it is withdrawing.
//
// Below the tables is the one bound ElevenLabs varies by what a customer pays:
// how many requests of a model may be in flight at once, given per plan and per
// group of models. A group is headed the way the page markets it rather than by
// identifier, so a column is matched to the models under it by the heading
// fragment they share, the realtime transcription column before the batch one
// that would otherwise take it. Enterprise is written as "Elevated" rather than
// as a figure and so carries no number, and neither does the free plan's
// allowance of no concurrent music generations at all, a figure of zero being
// indistinguishable from an absent one.
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
// The four capability guides document what the platform does rather than what a
// model is, and each states bounds and abilities the models page leaves out.
// Two kinds of sentence on one of them are facts about a model. The first names
// the models an ability arrived with, as "Audio Reference is available with
// Music v2", and is recorded against those. The guides never write an
// identifier, but ElevenLabs builds its identifiers out of exactly the words a
// reader says a model by, so a sentence names a model when it contains that
// model's identifier with the underscores spelled as spaces. The vendor's own
// prefix is dropped from some identifiers and not others, so Multilingual v2 is
// looked for as well as Eleven Multilingual v2, but only where dropping it
// leaves more than one word, since a bare v3 would match any sentence about any
// version of anything.
//
// The second kind states a bound or an ability of the capability itself and
// names no model: the longest recording the voice changer accepts in one
// request, the shortest and longest music a request produces, that a
// transcription request may carry a video as readily as a recording, and that
// the room a recording was made in can be taken out of the result. Those are
// recorded against every model of that kind, because the guide is that kind's
// documentation and the model is what implements it.
//
// The music guide opens with the sentences the flagship card uses, written of
// Eleven Music rather than of Eleven Music v2, and they are read as a card's
// bullets are. It is the only place the first version of the model is described
// at all. That guide also disagrees with the music endpoint about how long a
// generation may run, saying five minutes where the endpoint accepts a length
// of up to ten. The guide's figure is kept, being the one stated of the
// capability rather than of a request parameter, and the parameter's bound is
// not read at all.
//
// The endpoint references are read for the one thing they say that no other
// document does: which of an endpoint's models an ability belongs to.
// ElevenLabs writes that as a restriction on a request parameter, as "Only
// supported when using the eleven_ttv_v3 model", and only a parameter carrying
// such a restriction is
// read, because the restriction is what makes it a fact about a model rather
// than about the API. The parameter is recorded as the capability it names,
// never as its own name. The same pages state the longest text the voice design
// endpoint accepts, which belongs to both the models its model_id enumerates
// and is the same per-request character limit the models page states for speech
// synthesis.
//
// What else a reference states belongs to a model only where the page says
// which models it serves. Where model_id is an enumeration, as it is for voice
// design, sound effects and music, the route, the request parameters and the
// encodings the audio comes back in belong to exactly the models enumerated.
// Where it is a free string the page names its models by what they can do
// instead, and for two endpoints that is the same statement the identifiers
// make: text to speech takes any model with support for text to speech and the
// voice changer any model with support for speech to speech, so each is read
// onto every model of one kind. The speech to text reference is deliberately
// not, because it is the batch endpoint and the realtime model is served by a
// websocket, so its kind holds a model it does not serve.
//
// The parameters are recorded under their own key and never as capabilities. A
// capability list is not a list of what an API takes, which is why a parameter
// becomes a capability only where the page restricts it to one model.
//
// The API pricing page carries a second table below the cards, quoting each
// family against all six named plans. It is read for the rows the cards leave
// out: what ElevenLabs charges for the parts of a transcription request that
// are asked for separately, which are the same hour of audio billed again for
// having asked for more work on it and so carry a dimension rather than a
// metric of their own, and what training a music finetune costs, which is
// bought once where the audio it then produces is billed again. A row is read
// only where all six plans quote the same figure, so that a rate recorded with
// no plan on it is one the page states with no plan on it either. The rows
// repeating a card's rate are recorded again identically and dropped, which is
// how the claim that a rate does not vary by plan is checked rather than
// assumed.
//
// Neither table quotes voice design, the first generation multilingual model or
// the withdrawn transcription model, so those stay unpriced. The credits the
// help center counts a voice design generation in are a plan allowance and not
// a rate, and no page converts one into the other.
//
// Several things ElevenLabs publishes are not read here, because they say
// nothing about a model. The latency page explains what time to first audio is
// made of and quotes one figure, the same seventy-five milliseconds the Flash
// card already carries, against a family rather than a model. The changelog is
// a weekly page going back years whose entries are endpoint and SDK changes
// rather than model launches, so nothing carries a release date: no document
// read here dates a model. And the seven models nothing names stay nameless,
// because the models endpoint that would name them needs a key.
//
// A context window in tokens is not something any of these models has. Every
// one of them is a pairing of text and audio with no token vocabulary of its
// own, and the bound ElevenLabs does publish, on the characters one request may
// carry, is recorded under its own key rather than dressed up as one.
package elevenlabs
