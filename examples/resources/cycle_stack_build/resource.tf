resource "cycle_stack" "demo" {
  name = "Demo"

  source = {
    raw = jsonencode({
      version = "1.0"
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

# Builds are create-only. Changing about or instructions forces a new build.
# The provider waits until the build is live and does not deploy it.
resource "cycle_stack_build" "v1" {
  stack_id = cycle_stack.demo.id

  about = {
    description = "Initial build"
    version     = "1.0.0"
  }
}
