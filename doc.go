// Package stargazer defines types to watch a github repo for a given number of
// stars, and send a SMS message via Twilio.
//
// These two things aren't really related, and dividing them by functional
// responsibility here would be a better idea, but this started out as
// something that was supposed to take a very short time, and it ended up
// taking a much longer time, and for now I'm not going to worry about it.
package stargazer
