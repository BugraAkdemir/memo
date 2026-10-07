package app

import (
	"strings"
	"testing"
)

func TestClassifyImageIntent_Generate(t *testing.T) {
	for _, text := range []string{
		"bana bir kedi resmi çiz",
		"Kırmızı bir spor araba görseli oluştur",
		"şu manzaranın fotoğrafını üret",
		"kahve dükkanım için logo tasarla",
		"çiz bana bir ejderha",
		"bir dağ manzarası yap",
		"ÇİZ bana bir kale",
		"bir kedi resmi çizer misin?",
		"generate an image of a red car",
		"draw me a cat",
		"Draw a castle in the clouds",
		"make a logo for my coffee shop",
		"create a poster for a jazz night",
		"imagine a castle in the clouds",
		"show me a picture of a cat",
		"give me an image of a sunset over the sea",
		"I need a wallpaper of mountains",
		"can you make a picture of a dog?",
	} {
		if got := classifyImageIntent(text, false); got != imageIntentGenerate {
			t.Errorf("%q: got %v, want generate", text, got)
		}
	}
}

func TestClassifyImageIntent_NotAPictureRequest(t *testing.T) {
	for _, text := range []string{
		"",
		"merhaba nasılsın",
		"bu resmi açıkla",
		"what is in this picture?",
		"resim dosyasını nasıl küçültürüm",
		"how do I draw a circle in Python",
		"python ile resim çiz",
		"bir tablo oluştur",
		"excel tablosu yap",
		"grafik çiz",
		"draw a conclusion from these numbers",
		"nasıl resim çizilir",
		"how to make a logo",
		"create a docker image for my app",
		"make an image of the disk",
		"write a function that draws a rectangle",
		"yarın hava nasıl olacak",
		"resimlerim nerede",
		strings.Repeat("uzun bir belge ", 80) + " resim çiz",
	} {
		if got := classifyImageIntent(text, false); got != imageIntentNone {
			t.Errorf("%q: got %v, want none", text, got)
		}
	}
}

func TestClassifyImageIntent_WithAttachedPhoto(t *testing.T) {
	edit := []string{
		"make it black and white",
		"bunu anime yap",
		"arka planı kaldır",
		"remove the background",
		"add sunglasses",
		"gözlük ekle",
		"Ghibli tarzında yap",
		"bunun gibi bir logo tasarla",
		"turn this into a watercolor painting",
		"fix the lighting",
	}
	for _, text := range edit {
		if got := classifyImageIntent(text, true); got != imageIntentEdit {
			t.Errorf("%q + photo: got %v, want edit", text, got)
		}
	}
	describe := []string{
		"",
		"what is in this image?",
		"bunu açıkla",
		"bu fotoğraftaki yazıyı oku",
		"translate the text in this photo",
		"bu resimdeki kişi kim",
		"nerede çekilmiş olabilir?",
		"describe this",
	}
	for _, text := range describe {
		if got := classifyImageIntent(text, true); got != imageIntentNone {
			t.Errorf("%q + photo: got %v, want none (describe)", text, got)
		}
	}
}

func TestFoldForIntent(t *testing.T) {
	if got := foldForIntent("ÇİZ Şu ĞÜÖ ığ"); got != "ciz su guo ig" {
		t.Errorf("fold = %q", got)
	}
}
