package com.example;

public class Sample {
    private int value;

    public Sample(int value) {
        this.value = value;
    }

    public void process() {
        helper();
    }

    private void helper() {
        System.out.println("Processing: " + value);
    }

    public static void main(String[] args) {
        Sample sample = new Sample(10);
        sample.process();
    }
}

interface Worker {
    void doWork();
}
