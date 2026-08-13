// Package together parses Together AI's model catalog into the catalog model.
//
// Together is a host rather than a lab: it serves other people's models, so
// every entry carries the organization that made it and an API string that is
// a path rather than a name. It publishes one page holding eight tables of
// eight different shapes, one per modality, and the shape is the only thing
// saying what a rate means. An image model is priced per megapixel with a
// default step count, a video model per video at a fixed resolution and
// duration, an audio model per million characters or per minute of audio
// depending on which direction it runs, and an embedding model per million
// tokens beside the width of the vector it returns.
//
// The chat table also reports two capabilities and the quantization a model is
// served at. The capabilities are a yes-or-no column each, and only the yeses
// are recorded: a model answering no to both carries no capability list, which
// is as close as the catalog comes to saying it has neither.
//
// What Together does not publish: a bound on output length, for any model, and
// any modality for a chat model. Its modality column appears on the audio
// table alone, where it says which direction the model runs in.
package together
