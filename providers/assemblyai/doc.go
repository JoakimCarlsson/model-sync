// Package assemblyai parses AssemblyAI's models page into the catalog model.
//
// AssemblyAI sells transcription only, so nothing it publishes is priced by
// token. Its two rates are both per hour, but they meter different things: a
// pre-recorded model is charged by the hour of audio submitted, and a
// streaming one by the hour the connection stays open whether or not audio is
// flowing. They are recorded as different metrics because they are not
// comparable, and a reader that treated both as "per hour" would understate a
// voice agent that holds a socket open between utterances.
//
// The models themselves are not in a table. They are MDX cards carrying a
// title and a list of capabilities, and only the rate tables are markdown, so
// the two are read separately and joined on the display name.
package assemblyai
