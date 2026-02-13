package com.example;

import java.util.List;
import java.util.ArrayList;

class Base {
    protected int baseValue;
}

interface Worker {
    void doWork();
}

public class Sample extends Base implements Worker {
    private List<String> items;
    private Worker helper;

    public Sample(Worker helper) {
        this.helper = helper;
        this.items = new ArrayList<>();
    }

    public void doWork() {
        helper.doWork();
        System.out.println("Items: " + items.size());
    }

    public void addItem(String item) {
        items.add(item);
    }
}
