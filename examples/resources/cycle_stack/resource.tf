# An inline stack spec. The spec is stored on create; building it is a
# separate cycle_stack_build resource.
resource "cycle_stack" "demo" {
  name       = "Demo"
  identifier = "demo"

  variables = {
    image_tag = "alpine"
  }

  source = {
    raw = jsonencode({
      version = "1.0"
      about = {
        description = "Minimal demo stack"
      }
      containers = {
        web = {
          name = "web"
          image = {
            name = "nginx"
            origin = {
              type = "docker-hub"
              details = {
                target = "nginx:alpine"
              }
            }
          }
        }
      }
    })
  }
}

# Source a stack from a public git repository.
resource "cycle_stack" "from_repo" {
  name = "From Repo"

  source = {
    git_repo = {
      url        = "https://github.com/example/cycle-stack.git"
      branch     = "master"
      stack_file = "cycle.json"
    }
  }
}

# A private repo using HTTP user/token credentials (token is sensitive).
resource "cycle_stack" "private_repo" {
  name = "Private Repo"

  source = {
    git_repo = {
      url    = "https://github.com/example/private-stack.git"
      branch = "main"

      auth = {
        http = {
          username = var.git_username
          token    = var.git_token
        }
      }
    }
  }
}
