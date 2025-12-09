Okay, so token counting.

I have two ideas at the moment:

1. Add a `CountToken()` method signature to the `LLMClient` interface, and that method should interact with the token-counting APIs from the provider. From that point we bubble up the token count to the TUI.
2. Raw count the tokens on the TUI. The token-counting logic would reside mostly in the `tui.go`, but how do we calculate tool result tokens?

For now my approach is the #1 approach. It makes much more sense, since the SDKs at the moment do provide APIs to count tokens.

For the display, the token counter should be displayed as `<percentage> <count>/168k`. We should limit it to 168k for now.

Another problem: After we send a message to the provider API in their native format, then we convert the message back to our type, then are we going to convert the message it to native type again to send to the token-counting APIs? That sounds inefficient.

But I think it is necessary. I need to count the tokens from the text-typed message, as well as tokens from the tool uses.
