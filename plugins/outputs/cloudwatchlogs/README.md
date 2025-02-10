# Amazon CloudWatch Logs Output Plugin

For each configured target (log group/stream), the output plugin maintains a queue for log events that it batches.
Once each batch is full or the flush interval is reached, the current batch is sent using the PutLogEvents API to Amazon CloudWatch.

When concurrency is enabled, the pusher uses a shared worker pool to allow multiple concurrent sends.
```
                               Target #1 (Log Group/Stream)                               ┌──Shared Worker Pool──┐
           ┌──────────────────────────────────────────────────────────────────┐           │                      │
           │                                                                  │           │    ┌──Worker 1──┐    │
           │    ┌────────Event Queue────────┐      ┌─────────Batch─────────┐  │           │    │ ┌────────┐ │    │
           │    │                           │      │ ┌───────────────────┐ │  │    ┌──────┼───►│ │ Sender │ │    │
           │    │ ┌───┐     ┌───┐┌───┐┌───┐ │      │ │                   │ │  │    │      │    │ └────────┘ │    │
AddEvent───│───►│ │ n │ ... │ 3 ││ 2 ││ 1 │ ├─────►│ │ PutLogEventsInput │ ├──┼────┤      │    └────────────┘    │
           │    │ └───┘     └───┘└───┘└───┘ │      │ │                   │ │  │    │      │                      │
           │    │                           │      │ └───────────────────┘ │  │    │      │    ┌──Worker 2──┐    │
           │    └───────────────────────────┘      └───────────────────────┘  │    │      │    │ ┌────────┐ │    │
           │                                                                  │    ┼──────┼───►│ │ Sender │ │    │
           └──────────────────────────────────────────────────────────────────┘    │      │    │ └────────┘ │    │
                                                                                   │      │    └────────────┘    │
                                                                                   │      │                      │
                               Target #2 (Log Group/Stream)                        │      │          .           │
           ┌──────────────────────────────────────────────────────────────────┐    │      │          .           │
           │                                                                  │    │      │          .           │
           │    ┌────────Event Queue────────┐      ┌─────────Batch─────────┐  │    │      │                      │
           │    │                           │      │ ┌───────────────────┐ │  │    │      │                      │
           │    │ ┌───┐     ┌───┐┌───┐┌───┐ │      │ │                   │ │  │    │      │    ┌──Worker n──┐    │
AddEvent───│───►│ │ n │ ... │ 3 ││ 2 ││ 1 │ ├─────►│ │ PutLogEventsInput │ ├──┼────┤      │    │ ┌────────┐ │    │
           │    │ └───┘     └───┘└───┘└───┘ │      │ │                   │ │  │    └──────┼───►│ │ Sender │ │    │
           │    │                           │      │ └───────────────────┘ │  │           │    │ └────────┘ │    │
           │    └───────────────────────────┘      └───────────────────────┘  │           │    └────────────┘    │
           │                                                                  │           │                      │
           └──────────────────────────────────────────────────────────────────┘           └──────────────────────┘
```
