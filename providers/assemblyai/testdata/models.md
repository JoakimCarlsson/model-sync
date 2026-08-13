> ## Documentation Index
> Fetch the complete documentation index at: https://assemblyai.com/docs/llms.txt
> Use this file to discover all available pages before exploring further.

# Models

export const LanguageTable = ({languages, columns = 3}) => {
  return <div className="grid gap-2" style={{
    gridTemplateColumns: `repeat(${columns}, 1fr)`
  }}>
      {languages.map(language => <div key={language.code} className="flex justify-between items-center">
          <span>{language.name}</span>
          <code className="text-sm bg-gray-100 dark:bg-white/10 text-gray-800 dark:text-gray-200 px-2 py-1 rounded">
            {language.code}
          </code>
        </div>)}
    </div>;
};

AssemblyAI offers several state-of-the-art speech recognition models, each optimized for different use cases. Choose the model that best fits your needs based on accuracy, latency, cost, and language requirements.

## Pre-recorded models

<CardGroup cols={2}>
  <Card title="Universal-3.5 Pro" icon="basketball" href="/docs/pre-recorded-audio/universal-3-5-pro">
    <ul style={{ fontSize: "14px", lineHeight: "2", paddingLeft: "20px", margin: 0 }}>
      <li>Highest accuracy, fastest model</li>
      <li>Supports 18 languages</li>
      <li>Native code switching</li>
      <li>Contextual prompting capabilities</li>
      <li>Keyterms prompting up to 1,000 words</li>
    </ul>
  </Card>

  <Card title="Universal-2" icon="globe" href="/docs/pre-recorded-audio/select-the-speech-model">
    <ul style={{ fontSize: "14px", lineHeight: "2", paddingLeft: "20px", margin: 0 }}>
      <li>High accuracy, low latency</li>
      <li>Support across 99 languages</li>
      <li>Keyterms prompting up to 200 words</li>
      <li>Code switching</li>
    </ul>
  </Card>
</CardGroup>

<Tip>
  We recommend [Universal-3.5 Pro](/docs/pre-recorded-audio/universal-3-5-pro) for pre-recorded audio transcription. It delivers the highest accuracy and fastest transcription out of the box, with optional contextual prompting support. Universal-3.5 Pro supports 18 languages, for anything outside that set, the system automatically falls back to Universal-2, giving you coverage across 99 languages total without any extra configuration.
</Tip>

## Streaming models

<CardGroup cols={2}>
  <Card title="Universal-3.5 Pro Streaming" icon="basketball" href="/docs/streaming/getting-started/transcribe-streaming-audio">
    <ul style={{ fontSize: "14px", lineHeight: "2", paddingLeft: "20px", margin: 0 }}>
      <li>Highest accuracy for voice agents</li>
      <li>Fastest word emissions</li>
      <li>Advanced prompting capabilities</li>
      <li>Keyterms prompting up to 100 words</li>
      <li>18 languages with native code switching</li>
    </ul>
  </Card>

  <Card title="Universal-Streaming Multilingual" icon="globe" href="/docs/streaming/getting-started/transcribe-streaming-audio">
    <ul style={{ fontSize: "14px", lineHeight: "2", paddingLeft: "20px", margin: 0 }}>
      <li>Good balance of speed and cost-effectiveness</li>
      <li>Multilingual real-time transcription</li>
      <li>Keyterms prompting up to 100 words</li>
      <li>6 languages: en, es, pt, de, fr, it</li>
    </ul>
  </Card>

  <Card title="Universal-Streaming English" icon="bolt" href="/docs/streaming/getting-started/transcribe-streaming-audio">
    <ul style={{ fontSize: "14px", lineHeight: "2", paddingLeft: "20px", margin: 0 }}>
      <li>Good balance of speed and cost-effectiveness</li>
      <li>English transcription</li>
      <li>Keyterms prompting up to 100 words</li>
      <li>Intelligent endpointing</li>
    </ul>
  </Card>
</CardGroup>

<Tip>
  We recommend [Universal-3.5 Pro Streaming](/docs/streaming/getting-started/transcribe-streaming-audio) for streaming transcription. It provides the highest accuracy with sub-300ms latency, native multilingual code switching, and advanced prompting support.
</Tip>

## Add-on models

Add-on models enhance transcription accuracy for specialized domains. They work alongside your chosen speech model and are billed separately.

<CardGroup cols={2}>
  <Card title="Medical Mode" icon="stethoscope" href="/docs/pre-recorded-audio/medical-mode">
    <ul style={{ fontSize: "14px", lineHeight: "2", paddingLeft: "20px", margin: 0 }}>
      <li>Improved accuracy for medical terminology</li>
      <li>Medications, procedures, conditions, and dosages</li>
      <li>Works with pre-recorded and streaming models</li>
      <li>4 languages: en, es, de, fr</li>
    </ul>
  </Card>
</CardGroup>

### Medical Mode

Medical Mode (`domain: "medical-v1"`) is an add-on that enhances transcription accuracy for medical terminology — including medication names, procedures, conditions, and dosages. It is optimized for medical entity recognition to correct terms that other models frequently get wrong.

**Supported models:**

* Pre-recorded: Universal-3.5 Pro, Universal-2
* Streaming: Universal-3.5 Pro Streaming, Universal-Streaming English, Universal-Streaming Multilingual

**Supported languages:** English, Spanish, German, French

<Info>
  Medical Mode is billed as a separate add-on. See the [pricing page](https://www.assemblyai.com/pricing) for details.
</Info>

Learn more: [Medical Mode for pre-recorded audio](/docs/pre-recorded-audio/medical-mode) | [Medical Mode for streaming](/docs/streaming/medical-mode)

## Choosing the right model

### Pre-recorded

#### Universal-3.5 Pro

Universal-3.5 Pro is our most powerful Voice AI model, designed to capture the "hard stuff" that traditional ASR models struggle with. It delivers state-of-the-art accuracy for entities, rare words, and domain-specific terminology out of the box, with code switching and optional prompting for more control. It's also our fastest model, so you get the best accuracy without sacrificing speed.

**Best for:**

* Applications requiring highest-accuracy transcription
* Medical scribes needing clinical grade transcription accuracy
* Sales intelligence / Call centers needing native code-switching
* Meeting notetakers / recruiting notetakers needing high-quality diarization

<Accordion title="Supported languages">
  <LanguageTable
    languages={[
  { name: "English", code: "en" },
  { name: "Spanish", code: "es" },
  { name: "German", code: "de" },
  { name: "French", code: "fr" },
  { name: "Portuguese", code: "pt" },
  { name: "Italian", code: "it" },
  { name: "Turkish", code: "tr" },
  { name: "Dutch", code: "nl" },
  { name: "Swedish", code: "sv" },
  { name: "Norwegian", code: "no" },
  { name: "Danish", code: "da" },
  { name: "Finnish", code: "fi" },
  { name: "Hindi", code: "hi" },
  { name: "Vietnamese", code: "vi" },
  { name: "Arabic", code: "ar" },
  { name: "Hebrew", code: "he" },
  { name: "Japanese", code: "ja" },
  { name: "Mandarin", code: "zh" },
]}
    columns={2}
  />

  <br />
</Accordion>

<Note>
  **Regional dialects**

  Universal-3.5 Pro also supports regional dialects and local speech variants out of the box — no special configuration needed. See the full list of [supported dialects](/docs/pre-recorded-audio/supported-languages#regional-dialects-and-variants).
</Note>

[Try Universal-3.5 Pro here](/docs/pre-recorded-audio/universal-3-5-pro)

#### Universal-2

Universal-2 offers accurate, cost-effective transcription across 99 languages with low latency. It supports code switching and optional keyterms prompting for domain-specific vocabulary (up to 200 words). Universal-2 is the go-to choice when you need reliable transcription across diverse languages.

**Best for:**

* High accuracy at lower cost with broad language support
* High-volume, price-sensitive batch transcription
* Support for over 99 languages
* Recommended fallback when a requested language isn't supported by Universal-3.5 Pro

<Accordion title="Supported languages">
  <LanguageTable
    languages={[
  { name: "Global English", code: "en" },
  { name: "Australian English", code: "en_au" },
  { name: "British English", code: "en_uk" },
  { name: "US English", code: "en_us" },
  { name: "Spanish", code: "es" },
  { name: "French", code: "fr" },
  { name: "German", code: "de" },
  { name: "Italian", code: "it" },
  { name: "Portuguese", code: "pt" },
  { name: "Dutch", code: "nl" },
  { name: "Hindi", code: "hi" },
  { name: "Japanese", code: "ja" },
  { name: "Chinese", code: "zh" },
  { name: "Finnish", code: "fi" },
  { name: "Korean", code: "ko" },
  { name: "Polish", code: "pl" },
  { name: "Russian", code: "ru" },
  { name: "Turkish", code: "tr" },
  { name: "Ukrainian", code: "uk" },
  { name: "Vietnamese", code: "vi" },
  { name: "Afrikaans", code: "af" },
  { name: "Albanian", code: "sq" },
  { name: "Amharic", code: "am" },
  { name: "Arabic", code: "ar" },
  { name: "Armenian", code: "hy" },
  { name: "Assamese", code: "as" },
  { name: "Azerbaijani", code: "az" },
  { name: "Bashkir", code: "ba" },
  { name: "Basque", code: "eu" },
  { name: "Belarusian", code: "be" },
  { name: "Bengali", code: "bn" },
  { name: "Bosnian", code: "bs" },
  { name: "Breton", code: "br" },
  { name: "Bulgarian", code: "bg" },
  { name: "Burmese", code: "my" },
  { name: "Catalan", code: "ca" },
  { name: "Croatian", code: "hr" },
  { name: "Czech", code: "cs" },
  { name: "Danish", code: "da" },
  { name: "Estonian", code: "et" },
  { name: "Faroese", code: "fo" },
  { name: "Galician", code: "gl" },
  { name: "Georgian", code: "ka" },
  { name: "Greek", code: "el" },
  { name: "Gujarati", code: "gu" },
  { name: "Haitian", code: "ht" },
  { name: "Hausa", code: "ha" },
  { name: "Hawaiian", code: "haw" },
  { name: "Hebrew", code: "he" },
  { name: "Hungarian", code: "hu" },
  { name: "Icelandic", code: "is" },
  { name: "Indonesian", code: "id" },
  { name: "Javanese", code: "jw" },
  { name: "Kannada", code: "kn" },
  { name: "Kazakh", code: "kk" },
  { name: "Khmer", code: "km" },
  { name: "Lao", code: "lo" },
  { name: "Latin", code: "la" },
  { name: "Latvian", code: "lv" },
  { name: "Lingala", code: "ln" },
  { name: "Lithuanian", code: "lt" },
  { name: "Luxembourgish", code: "lb" },
  { name: "Macedonian", code: "mk" },
  { name: "Malagasy", code: "mg" },
  { name: "Malay", code: "ms" },
  { name: "Malayalam", code: "ml" },
  { name: "Maltese", code: "mt" },
  { name: "Maori", code: "mi" },
  { name: "Marathi", code: "mr" },
  { name: "Mongolian", code: "mn" },
  { name: "Nepali", code: "ne" },
  { name: "Norwegian", code: "no" },
  { name: "Norwegian Nynorsk", code: "nn" },
  { name: "Occitan", code: "oc" },
  { name: "Panjabi", code: "pa" },
  { name: "Pashto", code: "ps" },
  { name: "Persian", code: "fa" },
  { name: "Romanian", code: "ro" },
  { name: "Sanskrit", code: "sa" },
  { name: "Serbian", code: "sr" },
  { name: "Shona", code: "sn" },
  { name: "Sindhi", code: "sd" },
  { name: "Sinhala", code: "si" },
  { name: "Slovak", code: "sk" },
  { name: "Slovenian", code: "sl" },
  { name: "Somali", code: "so" },
  { name: "Sundanese", code: "su" },
  { name: "Swahili", code: "sw" },
  { name: "Swedish", code: "sv" },
  { name: "Swiss German", code: "de_ch" },
  { name: "Tagalog", code: "tl" },
  { name: "Tajik", code: "tg" },
  { name: "Tamil", code: "ta" },
  { name: "Tatar", code: "tt" },
  { name: "Telugu", code: "te" },
  { name: "Thai", code: "th" },
  { name: "Tibetan", code: "bo" },
  { name: "Turkmen", code: "tk" },
  { name: "Urdu", code: "ur" },
  { name: "Uzbek", code: "uz" },
  { name: "Welsh", code: "cy" },
  { name: "Yiddish", code: "yi" },
  { name: "Yoruba", code: "yo" },
]}
    columns={2}
  />

  <br />
</Accordion>

[Try Universal-2 here](/docs/pre-recorded-audio/select-the-speech-model)

### Streaming

#### Universal-3.5 Pro Streaming

The most accurate model with the fastest word emissions for voice agents that demand the highest quality. Best-in-class accuracy with advanced prompting capabilities, including both [keyterms prompting](/docs/streaming/prompting-and-keyterms) and [native prompting](/docs/streaming/prompting-and-keyterms). Supports English, Spanish, German, French, Portuguese, Italian, Turkish, Dutch, Swedish, Norwegian, Danish, Finnish, Hindi, Vietnamese, Arabic, Hebrew, Japanese, and Mandarin.

**Best for:**

* Real-time voice agents
* Applications requiring premium accuracy
* Customer service voice agents needing elite entity accuracy
* IVR replacement / binary response detection in short utterances
* Agent assist and sales intelligence needing real-time speaker diarization, mid-session dynamic prompting
* Multilingual voice agents with native code-switching across 18 languages
* Compliance and verbatim recording — disfluency control via prompting

<Accordion title="Supported languages">
  <LanguageTable
    languages={[
  { name: "English", code: "en" },
  { name: "Spanish", code: "es" },
  { name: "German", code: "de" },
  { name: "French", code: "fr" },
  { name: "Portuguese", code: "pt" },
  { name: "Italian", code: "it" },
  { name: "Turkish", code: "tr" },
  { name: "Dutch", code: "nl" },
  { name: "Swedish", code: "sv" },
  { name: "Norwegian", code: "no" },
  { name: "Danish", code: "da" },
  { name: "Finnish", code: "fi" },
  { name: "Hindi", code: "hi" },
  { name: "Vietnamese", code: "vi" },
  { name: "Arabic", code: "ar" },
  { name: "Hebrew", code: "he" },
  { name: "Japanese", code: "ja" },
  { name: "Mandarin", code: "zh" },
]}
    columns={2}
  />

  <br />
</Accordion>

<Note>
  **Regional dialects**

  Universal-3.5 Pro Streaming also supports regional dialects and local speech variants out of the box, with no special configuration needed. See the full list of [supported dialects](/docs/streaming/getting-started/transcribe-streaming-audio).
</Note>

[Learn more about Universal-3.5 Pro Streaming](/docs/streaming/getting-started/transcribe-streaming-audio)

#### Universal-Streaming Multilingual

A multilingual transcription model offering a good balance of speed and cost-effectiveness. Supports English, Spanish, German, French, Portuguese, and Italian. Features intelligent endpointing and [keyterms prompting](/docs/streaming/prompting-and-keyterms) support for up to 100 words.

**Best for:**

* Cost-effective real-time transcription across languages
* Cost-sensitive multilingual streaming across EN/ES/DE/FR/PT/IT

<Accordion title="Supported languages">
  <LanguageTable
    languages={[
  { name: "English", code: "en" },
  { name: "Spanish", code: "es" },
  { name: "German", code: "de" },
  { name: "French", code: "fr" },
  { name: "Portuguese", code: "pt" },
  { name: "Italian", code: "it" },
]}
    columns={2}
  />

  <br />
</Accordion>

[Learn more about Universal-Streaming Multilingual](/docs/streaming/getting-started/transcribe-streaming-audio)

#### Universal-Streaming English

An English transcription model offering a good balance of speed and cost-effectiveness. Features \~300ms word-by-word immutable transcripts, intelligent endpointing, and [keyterms prompting](/docs/streaming/prompting-and-keyterms) support for up to 100 words.

**Best for:**

* Cost-effective real-time transcription for English
* English-only real-time apps — fastest and cheapest streaming option for English

<Accordion title="Supported languages">
  <LanguageTable languages={[{ name: "English", code: "en" }]} columns={2} />

  <br />
</Accordion>

[Learn more about Universal-Streaming English](/docs/streaming/getting-started/transcribe-streaming-audio)

<Info>
  To learn how to specify a model, see
  [selecting a model for pre-recorded audio](/docs/pre-recorded-audio/select-the-speech-model)
  or [selecting a model for streaming audio](/docs/streaming/select-the-speech-model).
</Info>

## Pricing

For detailed pricing information, visit our [pricing page](https://www.assemblyai.com/pricing).

### Pre-recorded

| Model             | Price per Hour | Volume discounts |
| ----------------- | -------------- | ---------------- |
| Universal-3.5 Pro | \$0.21/hr      | Available        |
| Universal-2       | \$0.15/hr      | Available        |

### Streaming

Streaming is billed per hour of **session duration** — the total time your WebSocket connection stays open — not per hour of audio sent. See [Streaming Speech-to-Text billing](/docs/billing-and-pricing#streaming-speech-to-text-billing) for details.

| Model                            | Price per Hour (session duration) | Volume discounts |
| -------------------------------- | --------------------------------- | ---------------- |
| Universal-3.5 Pro Streaming      | \$0.45/hr                         | Available        |
| Universal-Streaming Multilingual | \$0.15/hr                         | Available        |
| Universal-Streaming English      | \$0.15/hr                         | Available        |

For volume discounts, please reach out to [sales@assemblyai.com](mailto:sales@assemblyai.com).

## Next steps

* Explore [Speech Understanding](/docs/speech-understanding) features like summarization, sentiment analysis, and more
* Learn about prompting: [Universal-3.5 Pro prompting guide](/docs/pre-recorded-audio/universal-3-5-pro/prompting) | [Universal-3.5 Pro Streaming prompting guide](/docs/streaming/prompting-and-keyterms)
