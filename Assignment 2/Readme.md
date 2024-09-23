a) What are packages in your implementation? What data structure do you use to transmit data and meta-data?

We use simple strings to represent the packages. Each message consists of space-separated values that convey the command (like "SYN", "SYN-ACK", or "ACK") and accompanying numerical values (like sequence numbers and acknowledgment numbers).

b) Does your implementation use threads or processes? Why is it not realistic to use threads?

No, our implementation does not use threads, since the server we are setting up acts as a link for exchanging information. This means that we are able to obtain the necessary information through the server. It is not realistic to use threads because the protocol should run across a network. This makes it able to connect through multiple devices.

c) In case the network changes the order in which messages are delivered, how would you handle message re-ordering?

We check the ack and seq number to make sure the message is delivered in the right order. In case of it gets the wrong number it will give an error.

You can handle message re-ordering by requesting the missing data, by giving the right sequence number, but we have not implemented that in our simulation.

d) In case messages can be delayed or lost, how does your implementation handle message loss?
As of right now we are not handling the delayed or lost messages but instead we print an error message in order to insinuate that there has been an error in delivering the message if the sequence number or the acknowledgement number is wrong.

e) Why is the 3-way handshake important?
The 3-way handshake ensures that there is a reliable data transfer between the client and server. It insures that they know that they are talking with each other.
