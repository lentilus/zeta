package server

import (
	"context"
	"log"
	"zeta/internal/cache"
	"zeta/internal/graph"
	"zeta/internal/resolver"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) workspaceExecuteCommand(
	context *glsp.Context,
	params *protocol.ExecuteCommandParams,
) (any, error) {
	if params.Command == "graph" {
		return nil, s.graph(context)
	}
	return nil, nil
}

func (s *Server) graph(ctx *glsp.Context) error {
	log.Println("called 'graph'")
	reuse := true
	if len(s.graphAddr) == 0 {
		s.graphAddr = graph.ShowGraph(":0")
		reuse = false
	}
	ctx.Notify(
		"window/showDocument",
		protocol.ShowDocumentParams{
			URI:      protocol.URI(s.graphAddr),
			External: &protocol.True,
		},
	)

	if reuse {
		return nil
	}

	updates, err := s.cache.Subscribe(context.Background())
	if err != nil {
		return err
	}

	go ProcessEvents(s, updates)
	return nil
}

func ProcessEvents(s *Server, events <-chan cache.Event) {
	idCounter := 0
	index := map[cache.Path]int{}
	pathToId := func(path cache.Path) int {
		if id, ok := index[path]; ok {
			return id
		}
		idCounter++
		index[path] = idCounter
		return idCounter
	}

	// noteToNode now uses note.Metadata directly, rather than fetching from cache
	noteToNode := func(note cache.NoteEvent) graph.Node {
		name := resolver.Title(note.Path, note.Metadata)
		node := graph.Node{
			Label:  name,
			Grayed: note.Placeholder,
			ID:     pathToId(note.Path),
		}
		return node
	}

	linkToLink := func(link cache.LinkEvent) graph.Link {
		return graph.Link{
			Source: pathToId(link.Source),
			Target: pathToId(link.Target),
		}
	}

	for ev := range events {
		switch ev.Type {
		case cache.CreateNote:
			if err := graph.AddNode(noteToNode(*ev.Note)); err != nil {
				log.Printf("graph.AddNode error: %v (event %+v)", err, ev)
			}

		case cache.UpdateNote:
			// Preserve the existing node ID, but update its label from the new Metadata
			id, _ := index[ev.Note.Path]
			delete(index, ev.Note.Path)
			ev.Note.Path = ev.Note.NewPath
			index[ev.Note.NewPath] = id

			// Use note.Metadata provided by the UpdateNote event
			updatedNode := graph.Node{
				Label:  resolver.Title(ev.Note.Path, ev.Note.Metadata),
				Grayed: ev.Note.Placeholder,
				ID:     id,
			}
			if err := graph.UpdateNode(updatedNode); err != nil {
				log.Printf("graph.UpdateNode error: %v (event %+v)", err, ev)
			}

		case cache.DeleteNote:
			if err := graph.DeleteNode(pathToId(ev.Note.Path)); err != nil {
				log.Printf("graph.DeleteNode error: %v (event %+v)", err, ev)
			}

		case cache.CreateLink:
			if err := graph.AddLink(linkToLink(*ev.Link)); err != nil {
				log.Printf("graph.AddLink error: %v (event %+v)", err, ev)
			}

		case cache.DeleteLink:
			if err := graph.DeleteLink(linkToLink(*ev.Link)); err != nil {
				log.Printf("graph.DeleteLink error: %v (event %+v)", err, ev)
			}

		default:
			log.Printf("unknown Operation %q in event %+v", ev.Type, ev)
		}
	}
}
