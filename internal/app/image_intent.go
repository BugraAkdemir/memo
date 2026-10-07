package app

import (
	"regexp"
	"strings"
	"unicode"
)

// imageIntent is what a chat message wants done with pictures.
type imageIntent int

const (
	// imageIntentNone: an ordinary chat turn — answer in text (a photo that came
	// with it is to be looked at and described, image-to-text).
	imageIntentNone imageIntent = iota
	// imageIntentGenerate: draw something new from the words (text-to-image).
	imageIntentGenerate
	// imageIntentEdit: change, restyle or build on the attached picture
	// (image-to-image).
	imageIntentEdit
)

// classifyImageIntent decides, from the words alone, whether a message asks for a
// picture to be made. It is deliberately conservative: a wrongly routed turn
// spends an image generation and shows a picture where the user wanted an
// answer, while a missed one only costs a re-ask with "/image …". Anything that
// reads like a question ABOUT pictures, a coding request, a table or a chart, or
// a pasted document is left to the text model.
//
// Turkish and English are both understood; the text is folded to ASCII first so
// "çiz", "ciz" and "ÇİZ" are one word.
func classifyImageIntent(text string, hasImage bool) imageIntent {
	t := foldForIntent(text)
	if t == "" || len([]rune(t)) > 500 {
		return imageIntentNone
	}

	if hasImage {
		// A photo with no words, or with words that ask about it, is to be read.
		if editAskRe.MatchString(t) {
			return imageIntentEdit
		}
		// "make a logo like this one" — the photo is a reference for a new picture.
		if wantsPictureMade(t) {
			return imageIntentEdit
		}
		return imageIntentNone
	}

	if wantsPictureMade(t) {
		return imageIntentGenerate
	}
	return imageIntentNone
}

// wantsPictureMade reports whether t (already folded) asks for a picture to be
// created. See classifyImageIntent.
func wantsPictureMade(t string) bool {
	if notAPictureRe.MatchString(t) {
		return false
	}
	if isHowToQuestion(t) {
		return false
	}
	// "draw me a …", "çiz bana …" — the verb itself carries the request.
	if drawVerbRe.MatchString(t) {
		return true
	}
	return pictureNounRe.MatchString(t) && makeVerbRe.MatchString(t)
}

// isHowToQuestion: "how do I draw …" / "nasıl resim çizilir" asks for knowledge,
// not for a picture. A polite request ("can you draw …", "çizer misin") is still
// a request.
func isHowToQuestion(t string) bool {
	if !howToStartRe.MatchString(t) {
		return false
	}
	return !politeRe.MatchString(t)
}

var (
	// Words that put a "draw/make/create" request in another world than pictures.
	notAPictureRe = regexp.MustCompile(`\b(python|javascript|typescript|java|golang|rust|c\+\+|html|css|svg|canvas|matplotlib|plot|plt|chart|graph|grafik|diagram|diyagram|flowchart|mermaid|ascii|table|tablo|tablosu|cizelge|excel|sheet|csv|markdown|sql|query|sorgu|kod|code|script|function|fonksiyon|library|kutuphane|framework|api|docker|container|konteyner|iso|disk|usb|partition|virtual machine|opencv|pillow|pil|react|flutter|widget|compose|dockerfile|sema|schema|uml|erd|wireframe|conclusion|comparison|analogy|attention|breath|salary|straw)\b`)

	howToStartRe = regexp.MustCompile(`^(how|what|why|where|when|which|who|nasil|neden|nicin|nerede|ne zaman|hangi|kim|ne )\b`)
	politeRe     = regexp.MustCompile(`\b(can you|could you|would you|will you|please|pls|lutfen|misin|musun|misiniz|musunuz|mi sin|mu sun|rica)\b`)

	// "draw me a …" and the Turkish "çiz" family. "draw" alone is not enough
	// ("draw a conclusion" is excluded above, "draw your attention" by needing the
	// article), so the English verbs want a following determiner or pronoun.
	drawVerbRe = regexp.MustCompile(`\b(draw|paint|sketch|illustrate|doodle) (me|us|a|an|the|my|our|some|this|that)\b|\bimagine (a|an|the)\b|\b(ciz|cizer|cizsene|cizebilir|cizip|cizin|boya|boyar)\b`)

	// Things that are pictures.
	pictureNounRe = regexp.MustCompile(`\b(resim|resmi|resmini|resimi|resimler|gorsel|gorseli|gorselini|gorseller|foto|fotoyu|fotograf|fotografi|fotografini|illustrasyon|cizim|cizimi|logo|logosu|logoyu|poster|posteri|afis|afisi|manzara|manzarasi|karikatur|karikaturu|avatar|avatari|ikon|ikonu|duvar kagidi|wallpaper|sticker|banner|kapak|image|images|picture|pictures|photo|photos|pic|illustration|drawing|painting|portrait|artwork|thumbnail|cover art|mockup|render|icon|selfie)\b`)

	// Words that ask for one to be made.
	makeVerbRe = regexp.MustCompile(`\b(yap|yapar misin|yapsana|yapabilir misin|uret|uretir misin|olustur|olusturur musun|tasarla|tasarlar misin|hazirla|yarat|goster|gosterir misin|cikar|istiyorum|isterim|lazim|olsun|ver|generate|create|make|design|produce|render|show me|give me|i need|i want|need|want|get me|build)\b`)

	// With a photo attached: asks to change it.
	editAskRe = regexp.MustCompile(`\b(duzenle|degistir|donustur|cevir|arka plan|arkaplan|arka planini|silueti|kaldir|cikar|sil|ekle|renklendir|siyah beyaz|siyah-beyaz|karakalem|karikatur|anime|studio ghibli|ghibli|pixar|boya|yagli boya|suluboya|stilinde|tarzinda|gibi yap|yap|olsun|rotus|temizle|netlestir|buyut|iyilestir|guzellestir|edit|change|modify|alter|turn (it|this|him|her|them)|make (it|this|him|her|them|the)|convert|transform|remove|erase|add|replace|swap|restyle|colorize|colourise|cartoon|sketch|in the style|style of|background|upscale|enhance|retouch|fix|clean up|black and white|b&w|paint it|draw on)\b`)
)

// foldForIntent lower-cases t and folds the Turkish letters (and their capitals)
// to ASCII so a single pattern covers every way of typing a word.
func foldForIntent(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case 'İ', 'I', 'ı', 'i':
			b.WriteByte('i')
		case 'ş', 'Ş':
			b.WriteByte('s')
		case 'ğ', 'Ğ':
			b.WriteByte('g')
		case 'ü', 'Ü':
			b.WriteByte('u')
		case 'ö', 'Ö':
			b.WriteByte('o')
		case 'ç', 'Ç':
			b.WriteByte('c')
		case '̇': // the combining dot a naive ToLower leaves on "İ"
		default:
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return strings.TrimSpace(b.String())
}
