# workon-issue
Gitlab helper to workon on an issue


## Usage

```bash
workon-issue 666
# your editor will open with issue description
# every save will update original issue
```

## Config file
example of config file

```bash
cat ~/.config/workon-issue/config.yml
```

```yaml
gitlab:
   url: https://gitlab.yourorganization.com
   repo: your/backlog
   token: YourPersonalToken

editor: emacsclient -n
```

## How to get a personal token
open in your gitlab instalation
https://gitlab.yourorganization.com/profile/personal_access_tokens
