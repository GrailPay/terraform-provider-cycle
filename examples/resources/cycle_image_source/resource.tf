# Pull an image from Docker Hub.
resource "cycle_image_source" "docker_hub" {
  name        = "nginx"
  type        = "direct"
  description = "Official nginx image from Docker Hub."

  origin = {
    docker_hub = {
      target = "nginx:1.27"
    }
  }
}

# Pull from a private registry (credentials are marked sensitive).
resource "cycle_image_source" "private_registry" {
  name = "internal-api"
  type = "direct"

  origin = {
    docker_registry = {
      target   = "myorg/internal-api:latest"
      url      = "registry.example.com"
      username = var.registry_username
      password = var.registry_password
    }
  }
}

# Build from a Dockerfile in a git repository.
resource "cycle_image_source" "from_repo" {
  name = "my-app"
  type = "direct"

  origin = {
    docker_file = {
      repo_url   = "https://github.com/myorg/my-app.git"
      branch     = "master"
      build_file = "Dockerfile"
    }
  }
}
