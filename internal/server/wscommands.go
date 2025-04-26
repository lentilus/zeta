package server

import (
	"log"
	"zeta/internal/cache"
	"zeta/internal/graph"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) graph(context *glsp.Context) error {
	log.Println("called 'graph'")
	reuse := true
	if len(s.graphAddr) == 0 {
		s.graphAddr = graph.ShowGraph(":0")
		reuse = false
	}
	context.Notify(
		"window/showDocument",
		protocol.ShowDocumentParams{
			URI:      protocol.URI(s.graphAddr),
			External: &protocol.True,
		},
	)

	if reuse {
		return nil
	}

	updates, _, err := s.cache.Subscribe()
	if err != nil {
		return err
	}

	go ProcessEvents(updates)
	return nil
}

func ProcessEvents(events <-chan cache.Event) {
	for ev := range events {
		switch ev.Operation {
		case "createNote":
			node := graph.Node{
				ID:     ev.Note.ID,
				Label:  ev.Note.Path,
				Grayed: ev.Note.Missing,
			}
			if err := graph.AddNode(node); err != nil {
				log.Printf("graph.AddNode error: %v (event %+v)", err, ev)
			}
		case "updateNote":
			node := graph.Node{
				ID:     ev.Note.ID,
				Label:  ev.Note.Path,
				Grayed: ev.Note.Missing,
			}
			if err := graph.UpdateNode(node); err != nil {
				log.Printf("graph.UpdateNode error: %v (event %+v)", err, ev)
			}

		case "deleteNote":
			if err := graph.DeleteNode(ev.Note.ID); err != nil {
				log.Printf("graph.DeleteNode error: %v (event %+v)", err, ev)
			}
		case "createLink":
			link := graph.Link{
				Source: ev.Link.SourceID,
				Target: ev.Link.TargetID,
			}

			if err := graph.AddLink(link); err != nil {
				log.Printf("graph.AddLink error: %v (event %+v)", err, ev)
			}
		case "deleteLink":
			link := graph.Link{
				Source: ev.Link.SourceID,
				Target: ev.Link.TargetID,
			}

			log.Println("Deleting Link")
			if err := graph.DeleteLink(link); err != nil {
				log.Printf("graph.DelteLink error: %v (event %+v)", err, ev)
			}
		default:
			log.Printf("unknown Operation %q in event %+v", ev.Operation, ev)
		}
	}
}
