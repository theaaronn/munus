# Munus

Terminal todo manager built with Bubble Tea. Todos stored as JSON files inside `*_munus` directories, each file containing an array of items with a title and body field. Switch between todo lists with number keys, navigate items with up/down arrows or j/k, quit with q or ctrl+c. Hierarchy: repository -> list -> todo.

Roadmap:
- DONE: Store text todos in local files (one json per todo)
- Encrypt local files
- Configure todo file location
- Integrate rg to look for todos (switch for title or body/details)
- Attach and open images and links
- Attach and play videos
- Get todos from SSH server (git repository) instead/and from local files
- Encrypt todos on the ssh server
- Configure which server to pull todos from (SSH keys?)
