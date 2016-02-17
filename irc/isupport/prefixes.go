package isupport

type Prefixes struct {
	Letters []rune
	Symbols []rune
}

func (p *Prefixes) LetterToSymbol(letter rune) (retval rune, ok bool) {
	for index, availableLetter := range p.Letters {
		if availableLetter == letter {
			ok = true
			retval = p.Symbols[index]
			return
		}
	}
	return
}

func (p *Prefixes) SymbolToLetter(symbol rune) (retval rune, ok bool) {
	for index, availableSymbol := range p.Symbols {
		if availableSymbol == symbol {
			ok = true
			retval = p.Symbols[index]
			return
		}
	}
	return
}
