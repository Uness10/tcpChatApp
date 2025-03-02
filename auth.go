package main

type userCredentials struct {
	pwd string
}

type authManager struct {
	users map[string]userCredentials
}
