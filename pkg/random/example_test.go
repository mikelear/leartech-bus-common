package random

// ExampleGenerator_NewString demonstrates generating a new random alphanumeric string using the Generator.
//
//nolint:testableexamples // Cannot predict the string output
func ExampleGenerator_NewString() {
	generator := NewGenerator()
	str, err := generator.NewString(10)
	if err != nil {
		panic(err)
	}
	println(str)
}

// ExampleGenerator_NewUUID demonstrates generating a new UUID using the Generator.
//
//nolint:testableexamples // Cannot predict the UUID output
func ExampleGenerator_NewUUID() {
	generator := NewGenerator()
	uuid := generator.NewUUID()
	println(uuid.String())
}
