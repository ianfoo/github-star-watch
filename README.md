# github stargazer 🤩  [![Build Status](https://travis-ci.org/ianfoo/github-stargazer.svg?branch=master)](https://travis-ci.org/ianfoo/github-stargazer)

All in the hopes of an [unsolicited back
massage](https://twitter.com/matryer/status/1042270822923030528) from [Mat
Ryer](https://github.com/matryer) when
[BitBar](https://github.com/matryer/bitbar) hits 10,000 stargazers.

### Really though, what is it?

This will watch any GitHub repo for any number of stars, and optionally send
you an SMS when the target is reached, and optionally star the repo when the
target is reached. And send you another SMS letting you know it's done that.

### Is that all it does?

Yes. Right now, anyway.

### Um, ok.

It's cool.

### Really?

Shut up.

## Fine. How do I use it?

First, get it. You'll need [Go](https://golang.org).
```bash
$ go get github.com/ianfoo/github-stargazer/...
```

### Set up environemnt for SMS

If you want to send SMS messages, you need a [Twilio](https://twilio.com)
account so that you can send SMS.  A [trial
account](https://support.twilio.com/hc/en-us/articles/223136107-How-does-Twilio-s-Free-Trial-work-)
will work fine. After you have that, set the following environment variables.

| Variable              | Set to                                                  |
|-----------------------|---------------------------------------------------------|
| `TWILIO_ACCOUNT_SID`  | Your Twilio Account SID                                 |
| `TWILIO_AUTH_TOKEN`   | Your secret Twilio auth token                           |
| `TWILIO_PHONE_NUMBER` | Your Twilio phone number that should send the SMS       |

### Set up environment for starring the repository

If you want to star the repository when the threshold is crossed, you'll need
to set `GITHUB_TOKEN` in your environment with a GitHub personal access token
that has the [`public_repo` OAuth
scope](https://developer.github.com/apps/building-oauth-apps/understanding-scopes-for-oauth-apps/).

### Run it
```bash
$ github-stargazer -phone 8005551212 -repo matryer/bitbar -target 9999
```

If you end up getting that unsolicited back massage, though, I'm gonna be
really cross with you.

## Complaints and how this could be much better

### There are no tests!

I know, I know. This was sort of stream of consiousness and done purely on
whimsy. This isn't production quality code, and I don't actually recommend you
(or I) write code in this fashion. Tests should be developed at least in step
with the base functionality. It promotes writing simpler, more testable code,
especially if you write the tests first. I dove in head first, and I'm sorry.
In having performed a bit of refactoring, I can tell you that it would have
been a little easier to get things back to a working state if I'd had tests to
run. I was going to write tests as part of this refactor, but it's getting
really late and it's only Wednesday. Well, Thursday morning now.

In any case, maybe this will become more generally useful at some point, and
have thousands of stars, but at this point it was just a diversion and an
excuse to poke around Twilio and GitHub's APIs.  It could be improved in many
ways, some of which I am aware of right now, and others I'm sure I would never
think of.

### What ways can you think of that it could be improved right now?

I have a few ideas, from the banal to the grandiose to the impractical.

* Some testing.
* Give error handling some actual thought and improve the slapdash job done
  here.
* Better logging: forcing starwatcher package to use zap.SugaredLogger is too
  specific?
* Separate the orthogonal concerns of GitHub and Twilio interaction.
* Better architecture overall.
* Follow-up checking on Twilio message status/delivery.
* Graceful shutdown.
* Other message transports, e.g., FB Messenger, Twitter.
* Tweet at the repo maintainer after auto-starring.
* Persistence to keep state across restarts.
* Ability to add multiple gazers.
* A dashboard to watch status.
* A CLI to add/remove/update gazers.
* Multi-user support.
* Support more than just stargazers.

If you have other ideas and are so motivated, file issues and/or PRs!
