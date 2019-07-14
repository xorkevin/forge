package validation

const templateValidate = `
func (s {{.Ident}}) {{.Prefix}}() error {
	return nil
}
`
