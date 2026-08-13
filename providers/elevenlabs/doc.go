// Package elevenlabs parses ElevenLabs' models page into the catalog model.
//
// ElevenLabs bills in credits drawn from a monthly plan rather than in dollars
// per unit, so its models carry no prices. That is what ElevenLabs publishes,
// not a gap here: there is no per-model dollar rate to record.
//
// What it does publish is a model's languages and, for speech synthesis, the
// longest text a single request may carry, which differs eightfold across the
// range. Its identifiers also encode what a model does — a name containing sts
// changes one voice into another, ttv designs a voice from a description,
// scribe transcribes — so the kind is read from the identifier.
package elevenlabs
