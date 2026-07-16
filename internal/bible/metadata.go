package bible

import "strings"

type canonical struct {
	ID, EnName, EnAbbr, IdName, IdAbbr, Testament string
	Chapters                                     int
}

// canon: 66 books, English + Indonesian names and abbreviations, chapter counts.
var canon = []canonical{
	{"gen", "Genesis", "Gen", "Kejadian", "Kej", "OT", 50},
	{"exod", "Exodus", "Ex", "Keluaran", "Kel", "OT", 40},
	{"lev", "Leviticus", "Lev", "Imamat", "Im", "OT", 27},
	{"num", "Numbers", "Num", "Bilangan", "Bil", "OT", 36},
	{"deut", "Deuteronomy", "Deut", "Ulangan", "Ul", "OT", 34},
	{"josh", "Joshua", "Josh", "Yosua", "Yos", "OT", 24},
	{"judg", "Judges", "Judg", "Hakim-Hakim", "Hak", "OT", 21},
	{"ruth", "Ruth", "Ruth", "Rut", "Rut", "OT", 4},
	{"1sam", "1 Samuel", "1Sam", "1 Samuel", "1Sam", "OT", 31},
	{"2sam", "2 Samuel", "2Sam", "2 Samuel", "2Sam", "OT", 24},
	{"1kgs", "1 Kings", "1Kgs", "1 Raja-Raja", "1Raj", "OT", 22},
	{"2kgs", "2 Kings", "2Kgs", "2 Raja-Raja", "2Raj", "OT", 25},
	{"1chr", "1 Chronicles", "1Chr", "1 Tawarikh", "1Taw", "OT", 29},
	{"2chr", "2 Chronicles", "2Chr", "2 Tawarikh", "2Taw", "OT", 36},
	{"ezra", "Ezra", "Ezra", "Ezra", "Ezr", "OT", 10},
	{"neh", "Nehemiah", "Neh", "Nehemia", "Neh", "OT", 13},
	{"esth", "Esther", "Esth", "Ester", "Est", "OT", 10},
	{"job", "Job", "Job", "Ayub", "Ayub", "OT", 42},
	{"ps", "Psalms", "Ps", "Mazmur", "Maz", "OT", 150},
	{"prov", "Proverbs", "Prov", "Amsal", "Ams", "OT", 31},
	{"eccl", "Ecclesiastes", "Eccl", "Pengkhotbah", "Pkh", "OT", 12},
	{"song", "Song of Solomon", "Song", "Kidung Agung", "Kid", "OT", 8},
	{"isa", "Isaiah", "Isa", "Yesaya", "Yes", "OT", 66},
	{"jer", "Jeremiah", "Jer", "Yeremia", "Yer", "OT", 52},
	{"lam", "Lamentations", "Lam", "Ratapan", "Rat", "OT", 5},
	{"ezek", "Ezekiel", "Ezek", "Yehezkiel", "Yeh", "OT", 48},
	{"dan", "Daniel", "Dan", "Daniel", "Dan", "OT", 12},
	{"hos", "Hosea", "Hos", "Hosea", "Hos", "OT", 14},
	{"joel", "Joel", "Joel", "Yoel", "Yoel", "OT", 3},
	{"amos", "Amos", "Amos", "Amos", "Amos", "OT", 9},
	{"obad", "Obadiah", "Obad", "Obaja", "Oba", "OT", 1},
	{"jonah", "Jonah", "Jonah", "Yunus", "Yun", "OT", 4},
	{"mic", "Micah", "Mic", "Mikha", "Mik", "OT", 7},
	{"nah", "Nahum", "Nah", "Nahum", "Nah", "OT", 3},
	{"hab", "Habakkuk", "Hab", "Habakuk", "Hab", "OT", 3},
	{"zeph", "Zephaniah", "Zeph", "Zefanya", "Zef", "OT", 3},
	{"hag", "Haggai", "Hag", "Hagai", "Hag", "OT", 2},
	{"zech", "Zechariah", "Zech", "Zakharia", "Zak", "OT", 14},
	{"mal", "Malachi", "Mal", "Maleakhi", "Mal", "OT", 4},
	{"matt", "Matthew", "Matt", "Matius", "Mat", "NT", 28},
	{"mark", "Mark", "Mark", "Markus", "Mrk", "NT", 16},
	{"luke", "Luke", "Luke", "Lukas", "Luk", "NT", 24},
	{"john", "John", "John", "Yohanes", "Yoh", "NT", 21},
	{"acts", "Acts", "Acts", "Kisah Para Rasul", "Kis", "NT", 28},
	{"rom", "Romans", "Rom", "Roma", "Rom", "NT", 16},
	{"1cor", "1 Corinthians", "1Cor", "1 Korintus", "1Kor", "NT", 16},
	{"2cor", "2 Corinthians", "2Cor", "2 Korintus", "2Kor", "NT", 13},
	{"gal", "Galatians", "Gal", "Galatia", "Gal", "NT", 6},
	{"eph", "Ephesians", "Eph", "Efesus", "Ef", "NT", 6},
	{"phil", "Philippians", "Phil", "Filipi", "Flp", "NT", 4},
	{"col", "Colossians", "Col", "Kolose", "Kol", "NT", 4},
	{"1thess", "1 Thessalonians", "1Thess", "1 Tesalonika", "1Tes", "NT", 5},
	{"2thess", "2 Thessalonians", "2Thess", "2 Tesalonika", "2Tes", "NT", 3},
	{"1tim", "1 Timothy", "1Tim", "1 Timotius", "1Tim", "NT", 6},
	{"2tim", "2 Timothy", "2Tim", "2 Timotius", "2Tim", "NT", 4},
	{"titus", "Titus", "Titus", "Titus", "Tit", "NT", 3},
	{"phlm", "Philemon", "Phlm", "Filemon", "Flm", "NT", 1},
	{"heb", "Hebrews", "Heb", "Ibrani", "Ibr", "NT", 13},
	{"jas", "James", "Jas", "Yakobus", "Yak", "NT", 5},
	{"1pet", "1 Peter", "1Pet", "1 Petrus", "1Pet", "NT", 5},
	{"2pet", "2 Peter", "2Pet", "2 Petrus", "2Pet", "NT", 3},
	{"1john", "1 John", "1John", "1 Yohanes", "1Yoh", "NT", 5},
	{"2john", "2 John", "2John", "2 Yohanes", "2Yoh", "NT", 1},
	{"3john", "3 John", "3John", "3 Yohanes", "3Yoh", "NT", 1},
	{"jude", "Jude", "Jude", "Yudas", "Yud", "NT", 1},
	{"rev", "Revelation", "Rev", "Wahyu", "Wah", "NT", 22},
}

func lookupByID(id string) (canonical, bool) {
	for _, b := range canon {
		if b.ID == id {
			return b, true
		}
	}
	return canonical{}, false
}

// ResolveBookID resolves an id, English name/abbr, or Indonesian name/abbr
// (case-insensitive) to the canonical lowercase book id.
func ResolveBookID(s string) (string, error) {
	l := strings.ToLower(strings.TrimSpace(s))
	for _, b := range canon {
		if l == strings.ToLower(b.ID) ||
			l == strings.ToLower(b.EnName) ||
			l == strings.ToLower(b.EnAbbr) ||
			l == strings.ToLower(b.IdName) ||
			l == strings.ToLower(b.IdAbbr) {
			return b.ID, nil
		}
	}
	return "", ErrNotFound
}

// IndonesianBookName returns the Indonesian name for a canonical id (used by
// the scrape adapter to build alkitab.mobi URLs). Empty string if unknown.
func IndonesianBookName(id string) string {
	if b, ok := lookupByID(id); ok {
		return b.IdName
	}
	return ""
}

// CanonicalBooks returns all 66 books as domain Book values (English names).
func CanonicalBooks() []Book {
	out := make([]Book, 0, len(canon))
	for _, b := range canon {
		out = append(out, Book{
			ID: b.ID, Name: b.EnName, Abbreviation: b.EnAbbr,
			Testament: b.Testament, Chapters: b.Chapters,
		})
	}
	return out
}
