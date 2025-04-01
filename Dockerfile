FROM scratch
COPY brainfuck/hello.bf /
CMD ["/hello.bf"]
