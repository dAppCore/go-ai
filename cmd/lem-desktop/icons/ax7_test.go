package icons

import . "dappco.re/go"

func TestIcons_Placeholder_Good(t *T) {
	icon := Placeholder()
	signature := []byte{0x89, 0x50, 0x4e, 0x47}
	got := icon[:4]

	AssertEqual(t, signature, got)
	AssertTrue(t, len(icon) > 0)
}

func TestIcons_Placeholder_Bad(t *T) {
	icon := Placeholder()
	got := len(icon)
	want := 0

	AssertTrue(t, got > want)
	AssertNotEqual(t, want, got)
}

func TestIcons_Placeholder_Ugly(t *T) {
	first := Placeholder()
	second := Placeholder()
	first[0] = 0

	AssertNotEqual(t, first[0], second[0])
	AssertEqual(t, byte(0x89), second[0])
}
