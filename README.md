# 900 words

Write daily.  900 words (or however many you want, really) to clear your
head of things.  Write them down, figure out what's going on in the back
of your mind.

900 words is more or less a simplified, single-user version of
<http://750words.com>.  (I'm not affiliated with them, other than liking
their site a lot.  But if you like this, and have $5 a month, you may
want to become a member there.)

## How to use this

- download the [latest release](https://github.com/heyLu/900words/releases/latest)
- run it

If you want, install it somewhere in your `$PATH` and/or start it by
adjusting the supplied [.service](./900words.service) file.  The steps
for that are approximately as following:

- download it
- `curl -LsfFO https://github.com/heyLu/900words/raw/master/900words.service`
- edit `900words.service` for your system
- `cp 900words.service ~/.config/systemd/user`
- `systemctl --user enable 900words`
- `systemctl --user start 900words`

Then it should be running on <http://localhost:9099>.

Have fun!

## License

Copyright © 2017 Lucas Stadler and Contributors

Distributed under the MIT license.
